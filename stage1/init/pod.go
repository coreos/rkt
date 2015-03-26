// Copyright 2014 CoreOS, Inc.
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

//+build linux

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema/types"
	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/coreos/go-systemd/unit"
	"github.com/coreos/rkt/common"
)

// Pod encapsulates a PodManifest and ImageManifests
type Pod struct {
	Root               string // root directory where the pod will be located
	UUID               types.UUID
	Manifest           *schema.PodManifest
	Apps               map[string]*schema.ImageManifest
	MetadataServiceURL string
	Networks           []string
}

var (
	defaultEnv = map[string]string{
		"PATH":    "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"SHELL":   "/bin/sh",
		"USER":    "root",
		"LOGNAME": "root",
		"HOME":    "/root",
	}
)

// LoadPod loads a Pod Manifest (as prepared by stage0) and
// its associated Application Manifests, under $root/stage1/opt/stage1/$apphash
func LoadPod(root string, uuid *types.UUID) (*Pod, error) {
	p := &Pod{
		Root: root,
		UUID: *uuid,
		Apps: make(map[string]*schema.ImageManifest),
	}

	buf, err := ioutil.ReadFile(common.PodManifestPath(p.Root))
	if err != nil {
		return nil, fmt.Errorf("failed reading pod manifest: %v", err)
	}

	pm := &schema.PodManifest{}
	if err := json.Unmarshal(buf, pm); err != nil {
		return nil, fmt.Errorf("failed unmarshalling pod manifest: %v", err)
	}
	p.Manifest = pm

	for _, app := range p.Manifest.Apps {
		ampath := common.ImageManifestPath(p.Root, app.Image.ID)
		buf, err := ioutil.ReadFile(ampath)
		if err != nil {
			return nil, fmt.Errorf("failed reading app manifest %q: %v", ampath, err)
		}

		am := &schema.ImageManifest{}
		if err = json.Unmarshal(buf, am); err != nil {
			return nil, fmt.Errorf("failed unmarshalling app manifest %q: %v", ampath, err)
		}
		name := am.Name.String()
		if _, ok := p.Apps[name]; ok {
			return nil, fmt.Errorf("got multiple definitions for app: %s", name)
		}
		p.Apps[name] = am
	}

	return p, nil
}

// quoteExec returns an array of quoted strings appropriate for systemd execStart usage
func quoteExec(exec []string) string {
	if len(exec) == 0 {
		// existing callers prefix {"/diagexec", "/app/root", "/work/dir", "/env/file"} so this shouldn't occur.
		panic("empty exec")
	}

	var qexec []string
	qexec = append(qexec, exec[0])
	// FIXME(vc): systemd gets angry if qexec[0] is quoted
	// https://bugs.freedesktop.org/show_bug.cgi?id=86171

	if len(exec) > 1 {
		for _, arg := range exec[1:] {
			escArg := strings.Replace(arg, `\`, `\\`, -1)
			escArg = strings.Replace(escArg, `"`, `\"`, -1)
			escArg = strings.Replace(escArg, `'`, `\'`, -1)
			escArg = strings.Replace(escArg, `$`, `$$`, -1)
			qexec = append(qexec, `"`+escArg+`"`)
		}
	}

	return strings.Join(qexec, " ")
}

func newUnitOption(section, name, value string) *unit.UnitOption {
	return &unit.UnitOption{Section: section, Name: name, Value: value}
}

// appToSystemd transforms the provided RuntimeApp+ImageManifest into systemd units
func (p *Pod) appToSystemd(ra *schema.RuntimeApp, am *schema.ImageManifest, interactive bool) error {
	name := ra.Name.String()
	id := ra.Image.ID
	app := am.App
	if ra.App != nil {
		app = ra.App
	}

	workDir := "/"
	if app.WorkingDirectory != "" {
		workDir = app.WorkingDirectory
	}

	env := app.Environment
	env.Set("AC_APP_NAME", name)
	env.Set("AC_METADATA_URL", p.MetadataServiceURL)

	if err := p.writeEnvFile(env, id); err != nil {
		return fmt.Errorf("unable to write environment file: %v", err)
	}

	// Create the Unix user and group
	var prefix string
	var username, groupname string

	if app.User == "root" || app.User == "0" {
		username = "root"
	} else {
		username = "u" + app.User
	}
	if app.Group == "root" || app.Group == "0" {
		groupname = "root"
	} else {
		// if app.User == app.Group, then systemd-sysusers requires that
		// username == groupname because it automatically creates a
		// group for the new user.
		groupname = "u" + app.Group
	}

	if err := os.MkdirAll(path.Join(common.Stage1RootfsPath(p.Root), "usr/lib/sysusers.d"), 0755); err != nil {
		return err
	}

	sysusersConfig := fmt.Sprintf("%su %s %s \"%s\"\n%sg %s %s\n",
		prefix, username, app.User, username,
		prefix, groupname, app.Group)

	err := ioutil.WriteFile(path.Join(common.Stage1RootfsPath(p.Root), "usr/lib/sysusers.d", ServiceUnitName(id)+".conf"),
		[]byte(sysusersConfig), 0640)
	if err != nil {
		return err
	}

	// - prepare-app bind mounted stage1's diagexec on "/.diagexec" in stage2
	// - use systemd option "RootDirectory" for chrooting the application,
	//   so diagexec does not have to chroot. Pass "" as chroot parameter.
	// - chdir to workDir
	// - since diagexec runs in the chrooted environment, it does not have
	//   access to the env file. Pass "" and let EnvionmentFile do the work.
	execWrap := []string{"/.diagexec", "", workDir, ""}

	execStart := quoteExec(append(execWrap, app.Exec...))
	opts := []*unit.UnitOption{
		newUnitOption("Unit", "Description", name),
		newUnitOption("Unit", "DefaultDependencies", "false"),
		newUnitOption("Unit", "OnFailureJobMode", "isolate"),
		newUnitOption("Unit", "OnFailure", "reaper.service"),
		newUnitOption("Unit", "Wants", "exit-watcher.service"),
		newUnitOption("Service", "Restart", "no"),
		newUnitOption("Service", "ExecStart", execStart),
		newUnitOption("Service", "User", username),
		newUnitOption("Service", "Group", groupname),
		newUnitOption("Service", "RootDirectory", common.RelAppRootfsPath(id)),
		newUnitOption("Service", "EnvironmentFile", RelEnvFilePath(id)),
	}

	if interactive {
		opts = append(opts, newUnitOption("Service", "StandardInput", "tty"))
		opts = append(opts, newUnitOption("Service", "StandardOutput", "tty"))
		opts = append(opts, newUnitOption("Service", "StandardError", "tty"))
	}

	for _, eh := range app.EventHandlers {
		var typ string
		switch eh.Name {
		case "pre-start":
			typ = "ExecStartPre"
		case "post-stop":
			typ = "ExecStopPost"
		default:
			return fmt.Errorf("unrecognized eventHandler: %v", eh.Name)
		}
		exec := quoteExec(append(execWrap, eh.Exec...))
		opts = append(opts, newUnitOption("Service", typ, exec))
	}

	saPorts := []types.Port{}
	for _, p := range app.Ports {
		if p.SocketActivated {
			saPorts = append(saPorts, p)
		}
	}

	if len(saPorts) > 0 {
		sockopts := []*unit.UnitOption{
			newUnitOption("Unit", "Description", name+" socket-activated ports"),
			newUnitOption("Unit", "DefaultDependencies", "false"),
			newUnitOption("Socket", "BindIPv6Only", "both"),
			newUnitOption("Socket", "Service", ServiceUnitName(id)),
		}

		for _, sap := range saPorts {
			var proto string
			switch sap.Protocol {
			case "tcp":
				proto = "ListenStream"
			case "udp":
				proto = "ListenDatagram"
			default:
				return fmt.Errorf("unrecognized protocol: %v", sap.Protocol)
			}
			sockopts = append(sockopts, newUnitOption("Socket", proto, fmt.Sprintf("%v", sap.Port)))
		}

		file, err := os.OpenFile(SocketUnitPath(p.Root, id), os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			return fmt.Errorf("failed to create socket file: %v", err)
		}
		defer file.Close()

		if _, err = io.Copy(file, unit.Serialize(sockopts)); err != nil {
			return fmt.Errorf("failed to write socket unit file: %v", err)
		}

		if err = os.Symlink(path.Join("..", SocketUnitName(id)), SocketWantPath(p.Root, id)); err != nil {
			return fmt.Errorf("failed to link socket want: %v", err)
		}

		opts = append(opts, newUnitOption("Unit", "Requires", SocketUnitName(id)))
	}

	opts = append(opts, newUnitOption("Unit", "Requires", InstantiatedPrepareAppUnitName(id)))
	opts = append(opts, newUnitOption("Unit", "After", InstantiatedPrepareAppUnitName(id)))

	opts = append(opts, newUnitOption("Unit", "Requires", "sysusers.service"))
	opts = append(opts, newUnitOption("Unit", "After", "sysusers.service"))

	file, err := os.OpenFile(ServiceUnitPath(p.Root, id), os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to create service unit file: %v", err)
	}
	defer file.Close()

	if _, err = io.Copy(file, unit.Serialize(opts)); err != nil {
		return fmt.Errorf("failed to write service unit file: %v", err)
	}

	if err = os.Symlink(path.Join("..", ServiceUnitName(id)), ServiceWantPath(p.Root, id)); err != nil {
		return fmt.Errorf("failed to link service want: %v", err)
	}

	return nil
}

// writeEnvFile creates an environment file for given app id
// the minimum required environment variables by the appc spec will be set to sensible
// defaults here if they're not provided by env.
func (p *Pod) writeEnvFile(env types.Environment, id types.Hash) error {
	ef := bytes.Buffer{}

	// Use the same format as systemd's EnvironmentFile option

	for dk, dv := range defaultEnv {
		if _, exists := env.Get(dk); !exists {
			fmt.Fprintf(&ef, "%s=%s\n", dk, dv)
		}
	}

	for _, e := range env {
		fmt.Fprintf(&ef, "%s=%s\n", e.Name, e.Value)
	}
	return ioutil.WriteFile(EnvFilePath(p.Root, id), ef.Bytes(), 0640)
}

// PodToSystemd creates the appropriate systemd service unit files for
// all the constituent apps of the Pod
func (p *Pod) PodToSystemd(interactive bool) error {
	for _, am := range p.Apps {
		ra := p.Manifest.Apps.Get(am.Name)
		if ra == nil {
			// should never happen
			panic("app not found in pod manifest")
		}
		if err := p.appToSystemd(ra, am, interactive); err != nil {
			return fmt.Errorf("failed to transform app %q into systemd service: %v", am.Name, err)
		}
	}

	return nil
}

// appToNspawnArgs transforms the given app manifest, with the given associated
// app image id, into a subset of applicable systemd-nspawn argument
func (p *Pod) appToNspawnArgs(ra *schema.RuntimeApp, am *schema.ImageManifest) ([]string, error) {
	args := []string{}
	name := ra.Name.String()
	id := ra.Image.ID
	app := am.App
	if ra.App != nil {
		app = ra.App
	}

	vols := make(map[types.ACName]types.Volume)

	// TODO(philips): this is implicitly creating a mapping from MountPoint
	// to volumes. This is a nice convenience for users but we will need to
	// introduce a --mount flag so they can control which mountPoint maps to
	// which volume.

	for _, v := range p.Manifest.Volumes {
		vols[v.Name] = v
	}

	for _, mp := range app.MountPoints {
		key := mp.Name
		vol, ok := vols[key]
		if !ok {
			return nil, fmt.Errorf("no volume for mountpoint %q in app %q", key, name)
		}
		opt := make([]string, 4)

		// If the readonly flag in the pod manifest is not nil,
		// then use it to override the readonly flag in the image manifest.
		readOnly := mp.ReadOnly
		if vol.ReadOnly != nil {
			readOnly = *vol.ReadOnly
		}

		if readOnly {
			opt[0] = "--bind-ro="
		} else {
			opt[0] = "--bind="
		}

		opt[1] = vol.Source
		opt[2] = ":"
		opt[3] = filepath.Join(common.RelAppRootfsPath(id), mp.Path)

		args = append(args, strings.Join(opt, ""))
	}

	for _, i := range am.App.Isolators {
		switch v := i.Value().(type) {
		case types.LinuxCapabilitiesSet:
			var caps []string
			// TODO: cleanup the API on LinuxCapabilitiesSet to give strings easily.
			for _, c := range v.Set() {
				caps = append(caps, string(c))
			}
			if i.Name == types.LinuxCapabilitiesRetainSetName {
				capList := strings.Join(caps, ",")
				args = append(args, "--capability="+capList)
			}
		}
	}

	return args, nil
}

// PodToNspawnArgs renders a prepared Pod as a systemd-nspawn
// argument list ready to be executed
func (p *Pod) PodToNspawnArgs() ([]string, error) {
	args := []string{
		"--uuid=" + p.UUID.String(),
		"--machine=" + "rkt-" + p.UUID.String(),
		"--directory=" + common.Stage1RootfsPath(p.Root),
	}

	for _, am := range p.Apps {
		ra := p.Manifest.Apps.Get(am.Name)
		if ra == nil {
			panic("could not find app in pod manifest!")
		}
		aa, err := p.appToNspawnArgs(ra, am)
		if err != nil {
			return nil, fmt.Errorf("failed to construct args for app %q: %v", am.Name, err)
		}
		args = append(args, aa...)
	}

	return args, nil
}
