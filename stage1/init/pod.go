// Copyright 2014 The rkt Authors
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
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
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
	Stage1Manifest     *schema.ImageManifest
	Stage1Options      *Stage1Option
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

const (
	nspawnEntrypoint     = "coreos.com/rkt/stage1/nspawn"
	versionEntrypoint    = "coreos.com/rkt/stage1/systemdversion"
	interpBinEntrypoint  = "coreos.com/rkt/stage1/ld.so"
	interpPathEntrypoint = "coreos.com/rkt/stage1/ld-path"
	shimEntrypoint       = "coreos.com/rkt/stage1/shim"
	// flavor could be dropped in favor of piggy-backing onto lack of shim ?!
	flavorEntrypoint = "coreos.com/rkt/stage1/flavor"
)

type Stage1Option struct {
	nspawnBin                     string
	systemdversion                string
	interpBin                     string
	interpPath                    string
	shimPath                      string
	flavor                        string
	systemdSupportsJournalLinking bool
}

// LoadPod loads a Pod Manifest (as prepared by stage0) and
// its associated Application Manifests, under $root/stage1/opt/stage1/$apphash
func LoadPod(root string, uuid *types.UUID) (*Pod, error) {
	var ok bool

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

	buf, err = ioutil.ReadFile(common.Stage1ManifestPath(p.Root))
	if err != nil {
		return nil, fmt.Errorf("failed reading stage1 manifest: %v", err)
	}

	s1m := &schema.ImageManifest{}
	if err := json.Unmarshal(buf, s1m); err != nil {
		return nil, fmt.Errorf("failed unmarshalling stage1 manifest: %v", err)
	}
	p.Stage1Manifest = s1m

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

	s1o := &Stage1Option{}
	// not mandatory, if using host binaries
	s1o.nspawnBin, _ = s1m.Annotations.Get(nspawnEntrypoint)
	// mandatory, but asserted in populateDefaults, because of usr-from-host flavor
	s1o.systemdversion, _ = s1m.Annotations.Get(versionEntrypoint)
	// not mandatory, if using host linker
	s1o.interpBin, _ = s1m.Annotations.Get(interpBinEntrypoint)
	s1o.interpPath, _ = s1m.Annotations.Get(interpPathEntrypoint)
	// not mandatory, if using patched systemd
	s1o.shimPath, _ = s1m.Annotations.Get(shimEntrypoint)
	s1o.flavor, ok = s1m.Annotations.Get(flavorEntrypoint)
	if !ok {
		return nil, fmt.Errorf("unable to determine stage1 flavor.")
	}
	p.Stage1Options = s1o

	if err = populateDefaults(p); err != nil {
		return nil, fmt.Errorf("unable to populate stage1 options: %v", err)
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

func (p *Pod) WritePrepareAppTemplate() error {
	opts := []*unit.UnitOption{
		newUnitOption("Unit", "Description", "Prepare minimum environment for chrooted applications"),
		newUnitOption("Unit", "DefaultDependencies", "false"),
		newUnitOption("Unit", "OnFailureJobMode", "fail"),
		newUnitOption("Service", "Type", "oneshot"),
		newUnitOption("Service", "Restart", "no"),
		newUnitOption("Service", "ExecStart", "/prepare-app %I"),
		newUnitOption("Service", "User", "0"),
		newUnitOption("Service", "Group", "0"),
		newUnitOption("Service", "CapabilityBoundingSet", "CAP_SYS_ADMIN CAP_DAC_OVERRIDE"),
	}

	if p.Stage1Options.systemdSupportsJournalLinking {
		opts = append(opts, newUnitOption("Unit", "Requires", "systemd-journald.service"))
		opts = append(opts, newUnitOption("Unit", "After", "systemd-journald.service"))
	}

	unitsPath := filepath.Join(common.Stage1RootfsPath(p.Root), unitsDir)
	file, err := os.OpenFile(filepath.Join(unitsPath, "prepare-app@.service"), os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to create service unit file: %v", err)
	}
	defer file.Close()

	if _, err = io.Copy(file, unit.Serialize(opts)); err != nil {
		return fmt.Errorf("failed to write service unit file: %v", err)
	}

	return nil
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

	// This is a partial implementation for app.User and app.Group:
	// For now, only numeric ids (and the string "root") are supported.
	var uid, gid int
	var err error
	if app.User == "root" {
		uid = 0
	} else {
		uid, err = strconv.Atoi(app.User)
		if err != nil {
			return fmt.Errorf("non-numerical user id not supported yet")
		}
	}
	if app.Group == "root" {
		gid = 0
	} else {
		gid, err = strconv.Atoi(app.Group)
		if err != nil {
			return fmt.Errorf("non-numerical group id not supported yet")
		}
	}

	execWrap := []string{"/diagexec", common.RelAppRootfsPath(id), workDir, RelEnvFilePath(id), strconv.Itoa(uid), strconv.Itoa(gid)}
	execStart := quoteExec(append(execWrap, app.Exec...))
	opts := []*unit.UnitOption{
		newUnitOption("Unit", "Description", name),
		newUnitOption("Unit", "DefaultDependencies", "false"),
		newUnitOption("Unit", "OnFailureJobMode", "isolate"),
		newUnitOption("Unit", "OnFailure", "reaper.service"),
		newUnitOption("Unit", "Wants", "exit-watcher.service"),
		newUnitOption("Service", "Restart", "no"),
		newUnitOption("Service", "ExecStart", execStart),
		newUnitOption("Service", "User", "0"),
		newUnitOption("Service", "Group", "0"),
	}

	if interactive {
		opts = append(opts, newUnitOption("Service", "StandardInput", "tty"))
		opts = append(opts, newUnitOption("Service", "StandardOutput", "tty"))
		opts = append(opts, newUnitOption("Service", "StandardError", "tty"))
	} else if p.Stage1Options.systemdSupportsJournalLinking {
		opts = append(opts, newUnitOption("Service", "StandardOutput", "journal+console"))
		opts = append(opts, newUnitOption("Service", "StandardError", "journal+console"))
		opts = append(opts, newUnitOption("Service", "SyslogIdentifier", filepath.Base(app.Exec[0])))
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

	for _, i := range am.App.Isolators {
		switch v := i.Value().(type) {
		case *types.ResourceMemory:
			limit := v.Limit().String()
			opts, err = maybeAddIsolator(opts, "memory", limit)
			if err != nil {
				return err
			}
		case *types.ResourceCPU:
			limit := v.Limit().String()
			opts, err = maybeAddIsolator(opts, "cpu", limit)
			if err != nil {
				return err
			}
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

	for dk, dv := range defaultEnv {
		if _, exists := env.Get(dk); !exists {
			fmt.Fprintf(&ef, "%s=%s\000", dk, dv)
		}
	}

	for _, e := range env {
		fmt.Fprintf(&ef, "%s=%s\000", e.Name, e.Value)
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
			catCmd := fmt.Sprintf("sudo rkt image cat-manifest --pretty-print %v", id)
			volumeCmd := ""
			for _, mp := range app.MountPoints {
				volumeCmd += fmt.Sprintf("--volume %s,kind=host,source=/some/path ", mp.Name)
			}

			return nil, fmt.Errorf("no volume for mountpoint %q in app %q.\n"+
				"You can inspect the volumes with:\n\t%v\n"+
				"App %q requires the following volumes:\n\t%v",
				key, name, catCmd, name, volumeCmd)
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
		"--machine=" + p.GetMachineID(),
		"--directory=" + common.Stage1RootfsPath(p.Root),
	}

	for _, am := range p.Apps {
		ra := p.Manifest.Apps.Get(am.Name)
		if ra == nil {
			panic("could not find app in pod manifest!")
		}
		aa, err := p.appToNspawnArgs(ra, am)
		if err != nil {
			return nil, err
		}
		args = append(args, aa...)
	}

	return args, nil
}

// GetAppHashes returns a list of hashes of the apps in this pod
func (p *Pod) GetAppHashes() []types.Hash {
	var names []types.Hash
	for _, a := range p.Manifest.Apps {
		names = append(names, a.Image.ID)
	}

	return names
}

// GetMachineID returns the machine id string of the pod to be passed to
// systemd-nspawn
func (p *Pod) GetMachineID() string {
	return "rkt-" + p.UUID.String()
}

func systemdSupportsJournalLinking(version string) bool {
	switch {
	case version == "v219":
		fallthrough
	case version == "v220":
		fallthrough
	case version == "master":
		return true
	default:
		return false
	}
}

func populateDefaults(p *Pod) error {
	switch p.Stage1Options.flavor {
	case "usr-from-host":
		hostNspawnBin, err := lookupPath("systemd-nspawn", os.Getenv("PATH"))
		if err != nil {
			return fmt.Errorf("unable to find host systemd-nspawn binary: %v", err)
		}
		p.Stage1Options.nspawnBin = hostNspawnBin

		// Check dynamically which version is installed on the host
		// Only support v220 for now.
		versionBytes, err := exec.Command(hostNspawnBin, "--version").CombinedOutput()
		if err != nil {
			return fmt.Errorf("unable to probe %s version: %v", hostNspawnBin, err)
		}
		version := strings.SplitN(strings.SplitN(string(versionBytes), "\n", 2)[0], " ", 2)
		if version[0] != "systemd" {
			return fmt.Errorf("%s version not supported: %v", hostNspawnBin, version)
		}
		p.Stage1Options.systemdversion = "v" + version[1]

		// Copy systemd, bash, etc. in stage1 at run-time
		if err := installAssets(); err != nil {
			return fmt.Errorf("cannot install assets from the host: %v", err)
		}
	default:
		p.Stage1Options.nspawnBin = filepath.Join(common.Stage1RootfsPath(p.Root), p.Stage1Options.nspawnBin)
	}

	if p.Stage1Options.systemdversion == "" {
		return fmt.Errorf("unable to determine stage1's systemd version.")
	}

	p.Stage1Options.systemdSupportsJournalLinking = systemdSupportsJournalLinking(p.Stage1Options.systemdversion)

	return nil
}
