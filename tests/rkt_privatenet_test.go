// Copyright 2015 The rkt Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/steveeJ/gexpect"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/vishvananda/netlink"
	"github.com/coreos/rkt/tests/testutils"
)

/*
 * No private net
 * ---
 * Container must have the same network namespace as the host
 */
func TestPrivateNetOmittedNetNS(t *testing.T) {
	testImageArgs := []string{"--exec=/inspect --print-netns"}
	testImage := patchTestACI("rkt-inspect-networking.aci", testImageArgs...)
	defer os.Remove(testImage)

	ctx := newRktRunCtx()
	defer ctx.cleanup()
	defer ctx.reset()

	cmd := fmt.Sprintf("%s --debug --insecure-skip-verify run --mds-register=false %s", ctx.cmd(), testImage)
	t.Logf("Command: %v\n", cmd)
	child, err := gexpect.Spawn(cmd)
	if err != nil {
		t.Fatalf("Cannot exec rkt: %v", err)
	}

	expectedRegex := `NetNS: (net:\[\d+\])`
	result, out, err := expectRegexWithOutput(child, expectedRegex)
	if err != nil {
		t.Fatalf("Error: %v\nOutput: %v", err, out)
	}

	ns, err := os.Readlink("/proc/self/ns/net")
	if err != nil {
		t.Fatalf("Cannot evaluate NetNS symlink: %v", err)
	}

	if nsChanged := ns != result[1]; nsChanged {
		t.Fatalf("container left host netns")
	}

	err = child.Wait()
	if err != nil {
		t.Fatalf("rkt didn't terminate correctly: %v", err)
	}
}

/*
 * No private net
 * ---
 * Container launches http server which must be reachable by the host via the
 * localhost address
 */
func TestPrivateNetOmittedConnectivity(t *testing.T) {

	httpPort, err := testutils.GetNextFreePort4(t)
	if err != nil {
		t.Fatalf("%v", err)
	}
	httpServeAddr := fmt.Sprintf("0.0.0.0:%v", httpPort)
	httpGetAddr := fmt.Sprintf("http://127.0.0.1:%v", httpPort)

	testImageArgs := []string{"--exec=/inspect --serve-http=" + httpServeAddr}
	testImage := patchTestACI("rkt-inspect-networking.aci", testImageArgs...)
	defer os.Remove(testImage)

	ctx := newRktRunCtx()
	defer ctx.cleanup()
	defer ctx.reset()

	cmd := fmt.Sprintf("%s --debug --insecure-skip-verify run --mds-register=false %s", ctx.cmd(), testImage)
	t.Logf("Command: %v\n", cmd)
	child, err := gexpect.Spawn(cmd)
	if err != nil {
		t.Fatalf("Cannot exec rkt: %v", err)
	}

	ga := testutils.NewGoroutineAssistant(t)
	// Child opens the server
	ga.Add(1)
	go func() {
		defer ga.Done()
		err = child.Wait()
		if err != nil {
			ga.Fatalf("rkt didn't terminate correctly: %v", err)
			return
		}
	}()

	// Host connects to the child
	ga.Add(1)
	go func() {
		defer ga.Done()
		expectedRegex := `serving on`
		_, out, err := expectRegexWithOutput(child, expectedRegex)
		if err != nil {
			ga.Fatalf("Error: %v\nOutput: %v", err, out)
			return
		}
		body, err := testutils.HttpGet(httpGetAddr)
		if err != nil {
			ga.Fatalf("%v\n", err)
			return
		}
		log.Printf("HTTP-Get received: %s", body)
	}()

	ga.Wait()
}

/*
 * Default private-net
 * ---
 * Container must be in a separate network namespace with private-net
 */
func TestPrivateNetDefaultNetNS(t *testing.T) {
	testImageArgs := []string{"--exec=/inspect --print-netns"}
	testImage := patchTestACI("rkt-inspect-networking.aci", testImageArgs...)
	defer os.Remove(testImage)

	ctx := newRktRunCtx()
	defer ctx.cleanup()
	defer ctx.reset()

	cmd := fmt.Sprintf("%s --debug --insecure-skip-verify run --private-net=default --mds-register=false %s", ctx.cmd(), testImage)
	t.Logf("Command: %v\n", cmd)
	child, err := gexpect.Spawn(cmd)
	if err != nil {
		t.Fatalf("Cannot exec rkt: %v", err)
	}

	expectedRegex := `NetNS: (net:\[\d+\])`
	result, out, err := expectRegexWithOutput(child, expectedRegex)
	if err != nil {
		t.Fatalf("Error: %v\nOutput: %v", err, out)
	}

	ns, err := os.Readlink("/proc/self/ns/net")
	if err != nil {
		t.Fatalf("Cannot evaluate NetNS symlink: %v", err)
	}

	if nsChanged := ns != result[1]; !nsChanged {
		t.Fatalf("container did not leave host netns")
	}

	err = child.Wait()
	if err != nil {
		t.Fatalf("rkt didn't terminate correctly: %v", err)
	}
}

/*
 * Default private-net
 * ---
 * Host launches http server on all interfaces in the host netns
 * Container must be able to connect via any IP address of the host in the
 * default network, which is NATed
 * TODO: test connection to host on an outside interface
 */
func TestPrivateNetDefaultConnectivity(t *testing.T) {
	httpPort, err := testutils.GetNextFreePort4(t)
	if err != nil {
		t.Fatalf("%v", err)
	}
	httpServeAddr := fmt.Sprintf("0.0.0.0:%v", httpPort)
	httpServeTimeout := 30

	nonLoIPv4, err := testutils.GetNonLoIfaceIPv4()
	if err != nil {
		t.Fatalf("%v", err)
	}
	if nonLoIPv4 == "" {
		t.Skipf("Can not find any NAT'able IPv4 on the host, skipping..")
	}

	httpGetAddr := fmt.Sprintf("http://%v:%v", nonLoIPv4, httpPort)
	t.Log("Telling the child to connect via", httpGetAddr)

	testImageArgs := []string{fmt.Sprintf("--exec=/inspect --get-http=%v", httpGetAddr)}
	testImage := patchTestACI("rkt-inspect-networking.aci", testImageArgs...)
	defer os.Remove(testImage)

	ctx := newRktRunCtx()
	defer ctx.cleanup()
	defer ctx.reset()

	ga := testutils.NewGoroutineAssistant(t)

	// Host opens the server
	ga.Add(1)
	go func() {
		defer ga.Done()
		err := testutils.HttpServe(httpServeAddr, httpServeTimeout)
		if err != nil {
			t.Fatalf("Error during HttpServe: %v", err)
		}
	}()

	// Child connects to host
	ga.Add(1)
	hostname, err := os.Hostname()
	go func() {
		defer ga.Done()
		cmd := fmt.Sprintf("%s --debug --insecure-skip-verify run --private-net=default --mds-register=false %s", ctx.cmd(), testImage)
		t.Logf("Command: %v\n", cmd)
		child, err := gexpect.Spawn(cmd)
		if err != nil {
			ga.Fatalf("Cannot exec rkt: %v", err)
			return
		}
		expectedRegex := `HTTP-Get received: (.*)\r`
		result, out, err := expectRegexWithOutput(child, expectedRegex)
		if err != nil {
			ga.Fatalf("Error: %v\nOutput: %v", err, out)
			return
		}
		if result[1] != hostname {
			ga.Fatalf("Hostname received by client `%v` doesn't match `%v`", result[1], hostname)
			return
		}

		err = child.Wait()
		if err != nil {
			ga.Fatalf("rkt didn't terminate correctly: %v", err)
		}
	}()

	ga.Wait()
}

/*
 * Default-restricted private-net
 * ---
 * Container launches http server on all its interfaces
 * Host must be able to connects to container's http server via container's
 * eth0's IPv4
 * TODO: verify that the container isn't NATed
 */
func TestPrivateNetDefaultRestrictedConnectivity(t *testing.T) {
	httpPort, err := testutils.GetNextFreePort4(t)
	if err != nil {
		t.Fatalf("%v", err)
	}
	httpServeAddr := fmt.Sprintf("0.0.0.0:%v", httpPort)
	iface := "eth0"

	testImageArgs := []string{fmt.Sprintf("--exec=/inspect --print-ipv4=%v --serve-http=%v", iface, httpServeAddr)}
	testImage := patchTestACI("rkt-inspect-networking.aci", testImageArgs...)
	defer os.Remove(testImage)

	ctx := newRktRunCtx()
	defer ctx.cleanup()
	defer ctx.reset()

	cmd := fmt.Sprintf("%s --debug --insecure-skip-verify run --private-net=default-restricted --mds-register=false %s", ctx.cmd(), testImage)
	t.Logf("Command: %v\n", cmd)
	child, err := gexpect.Spawn(cmd)
	if err != nil {
		t.Fatalf("Cannot exec rkt: %v", err)
	}

	expectedRegex := `IPv4: (.*)\r`
	result, out, err := expectRegexWithOutput(child, expectedRegex)
	if err != nil {
		t.Fatalf("Error: %v\nOutput: %v", err, out)
	}
	httpGetAddr := fmt.Sprintf("http://%v:%v", result[1], httpPort)

	ga := testutils.NewGoroutineAssistant(t)
	// Child opens the server
	ga.Add(1)
	go func() {
		defer ga.Done()
		err = child.Wait()
		if err != nil {
			ga.Fatalf("rkt didn't terminate correctly: %v", err)
		}
	}()

	// Host connects to the child
	ga.Add(1)
	go func() {
		defer ga.Done()
		expectedRegex := `serving on`
		_, out, err := expectRegexWithOutput(child, expectedRegex)
		if err != nil {
			ga.Fatalf("Error: %v\nOutput: %v", err, out)
			return
		}
		body, err := testutils.HttpGet(httpGetAddr)
		if err != nil {
			ga.Fatalf("%v\n", err)
			return
		}
		log.Printf("HTTP-Get received: %s", body)
		if err != nil {
			ga.Fatalf("%v\n", err)
		}
	}()

	ga.Wait()
}

func writeNetwork(t *testing.T, net networkTemplateT, netd string) error {
	var err error
	path := filepath.Join(netd, net.Name+".conf")
	file, err := os.Create(path)
	if err != nil {
		t.Errorf("%v", err)
	}

	b, err := json.Marshal(net)
	if err != nil {
		return err
	}

	fmt.Println("Writing", net.Name, "to", path)
	_, err = file.Write(b)
	if err != nil {
		return err
	}

	return nil
}

type networkTemplateT struct {
	Name      string
	Type      string
	Master    string `json:"master,omitempty"`
	IpMasq    bool
	IsGateway bool
	Ipam      ipamTemplateT
}

type ipamTemplateT struct {
	Type   string
	Subnet string              `json:"subnet,omitempty"`
	Routes []map[string]string `json:"routes,omitempty"`
}

func TestPrivateNetTemplates(t *testing.T) {
	net := networkTemplateT{
		Name: "ptp0",
		Type: "ptp",
		Ipam: ipamTemplateT{
			Type:   "host-local",
			Subnet: "10.1.3.0/24",
			Routes: []map[string]string{{"dst": "0.0.0.0/0"}},
		},
	}

	b, err := json.Marshal(net)
	if err != nil {
		t.Fatalf("%v", err)
	}
	expected := `{"Name":"ptp0","Type":"ptp","IpMasq":false,"IsGateway":false,"Ipam":{"Type":"host-local","subnet":"10.1.3.0/24","routes":[{"dst":"0.0.0.0/0"}]}}`
	if string(b) != expected {
		t.Fatalf("Template extected:\n%v\ngot:\n%v\n", expected, string(b))
	}
}

func prepareTestNet(t *testing.T, ctx *rktRunCtx, nt networkTemplateT) (netdir string) {
	configdir := ctx.directories[1].dir
	netdir = filepath.Join(configdir, "net.d")
	err := os.MkdirAll(netdir, 0644)
	if err != nil {
		t.Fatalf("Cannot create netdir: %v", err)
	}
	err = writeNetwork(t, nt, netdir)
	if err != nil {
		t.Fatalf("Cannot write network file: %v", err)
	}
	return netdir
}

/*
 * Two containers spawn in the same custom network.
 * ---
 * Container 1 opens the http server
 * Container 2 fires a HttpGet on it
 * The body of the HttpGet is Container 1's hostname, which must match
 */
func testPrivateNetCustomDual(t *testing.T, nt networkTemplateT) {
	httpPort, err := testutils.GetNextFreePort4(t)
	if err != nil {
		t.Fatalf("%v", err)
	}

	ctx := newRktRunCtx()
	defer ctx.cleanup()
	defer ctx.reset()

	netdir := prepareTestNet(t, ctx, nt)
	defer os.RemoveAll(netdir)

	container1IPv4, container1Hostname := make(chan string), make(chan string)
	ga := testutils.NewGoroutineAssistant(t)

	ga.Add(1)
	go func() {
		defer ga.Done()
		httpServeAddr := fmt.Sprintf("0.0.0.0:%v", httpPort)
		testImageArgs := []string{"--exec=/inspect --print-ipv4=eth0 --serve-http=" + httpServeAddr}
		testImage := patchTestACI("rkt-inspect-networking1.aci", testImageArgs...)
		defer os.Remove(testImage)

		cmd := fmt.Sprintf("%s --debug --insecure-skip-verify run --private-net=%v --mds-register=false %s", ctx.cmd(), nt.Name, testImage)
		fmt.Printf("Command: %v\n", cmd)
		child, err := gexpect.Spawn(cmd)
		if err != nil {
			ga.Fatalf("Cannot exec rkt: %v", err)
			return
		}

		expectedRegex := `IPv4: (\d+\.\d+\.\d+\.\d+)`
		result, out, err := expectRegexTimeoutWithOutput(child, expectedRegex, 30*time.Second)
		if err != nil {
			ga.Fatalf("Error: %v\nOutput: %v", err, out)
			return
		}
		container1IPv4 <- result[1]
		expectedRegex = `(rkt-.*): serving on`
		result, out, err = expectRegexTimeoutWithOutput(child, expectedRegex, 30*time.Second)
		if err != nil {
			ga.Fatalf("Error: %v\nOutput: %v", err, out)
			return
		}
		container1Hostname <- result[1]

		err = child.Wait()
		if err != nil {
			ga.Fatalf("rkt didn't terminate correctly: %v", err)
		}
	}()

	ga.Add(1)
	go func() {
		defer ga.Done()

		var httpGetAddr string
		httpGetAddr = fmt.Sprintf("http://%v:%v", <-container1IPv4, httpPort)

		testImageArgs := []string{"--exec=/inspect --get-http=" + httpGetAddr}
		testImage := patchTestACI("rkt-inspect-networking2.aci", testImageArgs...)
		defer os.Remove(testImage)

		cmd := fmt.Sprintf("%s --debug --insecure-skip-verify run --private-net=%v --mds-register=false %s", ctx.cmd(), nt.Name, testImage)
		fmt.Printf("Command: %v\n", cmd)
		child, err := gexpect.Spawn(cmd)
		if err != nil {
			t.Fatalf("Cannot exec rkt: %v", err)
		}

		expectedHostname := <-container1Hostname
		expectedRegex := `HTTP-Get received: (.*)\r`
		result, out, err := expectRegexTimeoutWithOutput(child, expectedRegex, 20*time.Second)
		if err != nil {
			ga.Fatalf("Error: %v\nOutput: %v", err, out)
			return
		}
		t.Logf("HTTP-Get received: %s", result[1])
		receivedHostname := result[1]

		if receivedHostname != expectedHostname {
			ga.Fatalf("Received hostname `%v` doesn't match `%v`", receivedHostname, expectedHostname)
			return
		}

		err = child.Wait()
		if err != nil {
			ga.Fatalf("rkt didn't terminate correctly: %v", err)
		}
	}()

	ga.Wait()
}

/*
 * Host launches http server on all interfaces in the host netns
 * Container must be able to connect via any IP address of the host in the
 * macvlan network, which is NAT
 * TODO: test connection to host on an outside interface
 */
func testPrivateNetCustomNatConnectivity(t *testing.T, nt networkTemplateT) {
	ctx := newRktRunCtx()
	defer ctx.cleanup()
	defer ctx.reset()

	netdir := prepareTestNet(t, ctx, nt)
	defer os.RemoveAll(netdir)

	httpPort, err := testutils.GetNextFreePort4(t)
	if err != nil {
		t.Fatalf("%v", err)
	}
	httpServeAddr := fmt.Sprintf("0.0.0.0:%v", httpPort)
	httpServeTimeout := 30

	nonLoIPv4, err := testutils.GetNonLoIfaceIPv4()
	if err != nil {
		t.Fatalf("%v", err)
	}
	if nonLoIPv4 == "" {
		t.Skipf("Can not find any NAT'able IPv4 on the host, skipping..")
	}

	httpGetAddr := fmt.Sprintf("http://%v:%v", nonLoIPv4, httpPort)
	t.Log("Telling the child to connect via", httpGetAddr)

	ga := testutils.NewGoroutineAssistant(t)

	// Host opens the server
	ga.Add(1)
	go func() {
		defer ga.Done()
		err := testutils.HttpServe(httpServeAddr, httpServeTimeout)
		if err != nil {
			t.Fatalf("Error during HttpServe: %v", err)
		}
	}()

	// Child connects to host
	ga.Add(1)
	hostname, err := os.Hostname()
	go func() {
		defer ga.Done()
		testImageArgs := []string{fmt.Sprintf("--exec=/inspect --get-http=%v", httpGetAddr)}
		testImage := patchTestACI("rkt-inspect-networking.aci", testImageArgs...)
		defer os.Remove(testImage)

		cmd := fmt.Sprintf("%s --debug --insecure-skip-verify run --private-net=%v --mds-register=false %s", ctx.cmd(), nt.Name, testImage)
		t.Logf("Command: %v\n", cmd)
		child, err := gexpect.Spawn(cmd)
		if err != nil {
			ga.Fatalf("Cannot exec rkt: %v", err)
			return
		}
		expectedRegex := `HTTP-Get received: (.*)\r`
		result, out, err := expectRegexWithOutput(child, expectedRegex)
		if err != nil {
			ga.Fatalf("Error: %v\nOutput: %v", err, out)
			return
		}
		if result[1] != hostname {
			ga.Fatalf("Hostname received by client `%v` doesn't match `%v`", result[1], hostname)
			return
		}

		err = child.Wait()
		if err != nil {
			ga.Fatalf("rkt didn't terminate correctly: %v", err)
		}
	}()

	ga.Wait()
}

func TestPrivateNetCustomPtp(t *testing.T) {
	nt := networkTemplateT{
		Name:   "ptp0",
		Type:   "ptp",
		IpMasq: true,
		Ipam: ipamTemplateT{
			Type:   "host-local",
			Subnet: "10.1.1.0/24",
			Routes: []map[string]string{
				{"dst": "0.0.0.0/0"},
			},
		},
	}
	testPrivateNetCustomNatConnectivity(t, nt)
	testPrivateNetCustomDual(t, nt)
}

func TestPrivateNetCustomMacvlan(t *testing.T) {
	iface, _, err := testutils.GetNonLoIfaceWithAddrs(netlink.FAMILY_V4)
	if err != nil {
		t.Fatalf("Error while getting non-lo host interface: %v\n", err)
	}
	if iface.Name == "" {
		t.Skipf("Cannot run test without non-lo host interface")
	}

	nt := networkTemplateT{
		Name:   "macvlan0",
		Type:   "macvlan",
		Master: iface.Name,
		Ipam: ipamTemplateT{
			Type:   "host-local",
			Subnet: "10.1.2.0/24",
		},
	}
	testPrivateNetCustomDual(t, nt)
}

func TestPrivateNetCustomBridge(t *testing.T) {
	iface, _, err := testutils.GetNonLoIfaceWithAddrs(netlink.FAMILY_V4)
	if err != nil {
		t.Fatalf("Error while getting non-lo host interface: %v\n", err)
	}
	if iface.Name == "" {
		t.Skipf("Cannot run test without non-lo host interface")
	}

	nt := networkTemplateT{
		Name:      "bridge0",
		Type:      "bridge",
		IpMasq:    true,
		IsGateway: true,
		Master:    iface.Name,
		Ipam: ipamTemplateT{
			Type:   "host-local",
			Subnet: "10.1.3.0/24",
			Routes: []map[string]string{
				{"dst": "0.0.0.0/0"},
			},
		},
	}
	testPrivateNetCustomNatConnectivity(t, nt)
	testPrivateNetCustomDual(t, nt)
}

func TestPrivateNetOverride(t *testing.T) {
	ctx := newRktRunCtx()
	defer ctx.cleanup()
	defer ctx.reset()

	iface, _, err := testutils.GetNonLoIfaceWithAddrs(netlink.FAMILY_V4)
	if err != nil {
		t.Fatalf("Error while getting non-lo host interface: %v\n", err)
	}
	if iface.Name == "" {
		t.Skipf("Cannot run test without non-lo host interface")
	}

	nt := networkTemplateT{
		Name:   "overridemacvlan",
		Type:   "macvlan",
		Master: iface.Name,
		Ipam: ipamTemplateT{
			Type:   "host-local",
			Subnet: "10.1.4.0/24",
		},
	}

	netdir := prepareTestNet(t, ctx, nt)
	defer os.RemoveAll(netdir)

	testImageArgs := []string{"--exec=/inspect --print-ipv4=eth0"}
	testImage := patchTestACI("rkt-inspect-networking1.aci", testImageArgs...)
	defer os.Remove(testImage)

	expectedIP := "10.1.4.244"

	cmd := fmt.Sprintf("%s --debug --insecure-skip-verify run --private-net=all --private-net=\"%s:IP=%s\" --mds-register=false %s", ctx.cmd(), nt.Name, expectedIP, testImage)
	fmt.Printf("Command: %v\n", cmd)
	child, err := gexpect.Spawn(cmd)
	if err != nil {
		t.Fatalf("Cannot exec rkt: %v", err)
		return
	}

	defer func() {
		err = child.Wait()
		if err != nil {
			t.Fatalf("rkt didn't terminate correctly: %v", err)
		}
	}()

	expectedRegex := `IPv4: (\d+\.\d+\.\d+\.\d+)`
	result, out, err := expectRegexTimeoutWithOutput(child, expectedRegex, 30*time.Second)
	if err != nil {
		t.Fatalf("Error: %v\nOutput: %v", err, out)
		return
	}

	containerIP := result[1]
	if expectedIP != containerIP {
		t.Fatalf("overriding IP did not work: Got %q but expected %q", containerIP, expectedIP)
	}
}
