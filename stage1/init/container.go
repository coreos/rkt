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

	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/appc/spec/schema"
	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/appc/spec/schema/types"
	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/coreos/go-systemd/unit"
	"github.com/coreos/rocket/common"
)

// Container encapsulates a ContainerRuntimeManifest and ImageManifests
type Container struct {
	Root           string // root directory where the container will be located
	Manifest       *schema.ContainerRuntimeManifest
	Apps           map[string]*schema.ImageManifest
	MetadataSvcURL string
	Networks       []string
}

// LoadContainer loads a Container Runtime Manifest (as prepared by stage0) and
// its associated Application Manifests, under $root/stage1/opt/stage1/$apphash
func LoadContainer(root string) (*Container, error) {
	c := &Container{
		Root: root,
		Apps: make(map[string]*schema.ImageManifest),
	}

	buf, err := ioutil.ReadFile(common.ContainerManifestPath(c.Root))
	if err != nil {
		return nil, fmt.Errorf("failed reading container runtime manifest: %v", err)
	}

	cm := &schema.ContainerRuntimeManifest{}
	if err := json.Unmarshal(buf, cm); err != nil {
		return nil, fmt.Errorf("failed unmarshalling container runtime manifest: %v", err)
	}
	c.Manifest = cm

	for _, app := range c.Manifest.Apps {
		ampath := common.ImageManifestPath(c.Root, app.ImageID)
		buf, err := ioutil.ReadFile(ampath)
		if err != nil {
			return nil, fmt.Errorf("failed reading app manifest %q: %v", ampath, err)
		}

		am := &schema.ImageManifest{}
		if err = json.Unmarshal(buf, am); err != nil {
			return nil, fmt.Errorf("failed unmarshalling app manifest %q: %v", ampath, err)
		}
		name := am.Name.String()
		if _, ok := c.Apps[name]; ok {
			return nil, fmt.Errorf("got multiple definitions for app: %s", name)
		}
		c.Apps[name] = am
	}

	return c, nil
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
			qexec = append(qexec, `"`+escArg+`"`)
		}
	}

	return strings.Join(qexec, " ")
}

func newUnitOption(section, name, value string) *unit.UnitOption {
	return &unit.UnitOption{Section: section, Name: name, Value: value}
}

// appMountsToSystemd writes a systemd mount unit file for the specified app
func (c *Container) appMountsToSystemd(am *schema.ImageManifest, id types.Hash) error {
	opts := []*unit.UnitOption{
		newUnitOption("Mount", "What", "/dev"),
		newUnitOption("Mount", "Where", common.RelAppRootfsPath(id)+"/dev"),
		newUnitOption("Mount", "Options", "bind"),
	}

	file, err := os.OpenFile(MountUnitPath(c.Root, id), os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to create mount unit file: %v", err)
	}
	defer file.Close()

	if _, err = io.Copy(file, unit.Serialize(opts)); err != nil {
		return fmt.Errorf("failed to write mount unit file: %v", err)
	}

	if err = os.Symlink(path.Join("..", MountUnitName(id)), MountWantPath(c.Root, id)); err != nil {
		return fmt.Errorf("failed to link mount want: %v", err)
	}

	return nil
}

// appToSystemd transforms the provided app manifest into systemd units
func (c *Container) appToSystemd(am *schema.ImageManifest, id types.Hash) error {
	name := am.Name.String()
	app := am.App

	workDir := "/"
	if app.WorkingDirectory != "" {
		workDir = app.WorkingDirectory
	}

	env := app.Environment
	env.Set("AC_APP_NAME", name)
	env.Set("AC_METADATA_URL", c.MetadataSvcURL)

	if err := c.writeEnvFile(env, id); err != nil {
		return fmt.Errorf("unable to write environment file: %v", err)
	}

	execWrap := []string{"/diagexec", common.RelAppRootfsPath(id), workDir, RelEnvFilePath(id)}
	execStart := quoteExec(append(execWrap, app.Exec...))
	opts := []*unit.UnitOption{
		newUnitOption("Unit", "Description", name),
		newUnitOption("Unit", "DefaultDependencies", "false"),
		newUnitOption("Unit", "OnFailureJobMode", "isolate"),
		newUnitOption("Unit", "OnFailure", "reaper.service"),
		newUnitOption("Unit", "Wants", "exit-watcher.service"),
		newUnitOption("Service", "Restart", "no"),
		newUnitOption("Service", "ExecStart", execStart),
		newUnitOption("Service", "User", app.User),
		newUnitOption("Service", "Group", app.Group),
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

		file, err := os.OpenFile(SocketUnitPath(c.Root, id), os.O_WRONLY|os.O_CREATE, 0644)
		if err != nil {
			return fmt.Errorf("failed to create socket file: %v", err)
		}
		defer file.Close()

		if _, err = io.Copy(file, unit.Serialize(sockopts)); err != nil {
			return fmt.Errorf("failed to write socket unit file: %v", err)
		}

		if err = os.Symlink(path.Join("..", SocketUnitName(id)), SocketWantPath(c.Root, id)); err != nil {
			return fmt.Errorf("failed to link socket want: %v", err)
		}

		opts = append(opts, newUnitOption("Unit", "Requires", SocketUnitName(id)))
	}

	err := c.appMountsToSystemd(am, id)
	if err != nil {
		return fmt.Errorf("failed to create mount unit file: %v", err)
	}
	opts = append(opts, newUnitOption("Unit", "Requires", MountUnitName(id)))

	file, err := os.OpenFile(ServiceUnitPath(c.Root, id), os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to create service unit file: %v", err)
	}
	defer file.Close()

	if _, err = io.Copy(file, unit.Serialize(opts)); err != nil {
		return fmt.Errorf("failed to write service unit file: %v", err)
	}

	if err = os.Symlink(path.Join("..", ServiceUnitName(id)), ServiceWantPath(c.Root, id)); err != nil {
		return fmt.Errorf("failed to link service want: %v", err)
	}

	return nil
}

// writeEnvFile creates an environment file for given app id
func (c *Container) writeEnvFile(env types.Environment, id types.Hash) error {
	ef := bytes.Buffer{}
	for _, e := range env {
		fmt.Fprintf(&ef, "%s=%s\000", e.Name, e.Value)
	}
	return ioutil.WriteFile(EnvFilePath(c.Root, id), ef.Bytes(), 0640)
}

// ContainerToSystemd creates the appropriate systemd service unit files for
// all the constituent apps of the Container
func (c *Container) ContainerToSystemd() error {
	for _, am := range c.Apps {
		a := c.Manifest.Apps.Get(am.Name)
		if a == nil {
			// should never happen
			panic("app not found in container manifest")
		}
		if err := c.appToSystemd(am, a.ImageID); err != nil {
			return fmt.Errorf("failed to transform app %q into systemd service: %v", am.Name, err)
		}
	}

	return nil
}

// appToNspawnArgs transforms the given app manifest, with the given associated
// app image id, into a subset of applicable systemd-nspawn argument
func (c *Container) appToNspawnArgs(am *schema.ImageManifest, id types.Hash) ([]string, error) {
	args := []string{}
	name := am.Name.String()

	vols := make(map[types.ACName]types.Volume)

	// TODO(philips): this is implicitly creating a mapping from MountPoint
	// to volumes. This is a nice convenience for users but we will need to
	// introduce a --mount flag so they can control which mountPoint maps to
	// which volume.

	for _, v := range c.Manifest.Volumes {
		vols[v.Name] = v
	}

	for _, mp := range am.App.MountPoints {
		key := mp.Name
		vol, ok := vols[key]
		if !ok {
			return nil, fmt.Errorf("no volume for mountpoint %q in app %q", key, name)
		}
		opt := make([]string, 4)

		if mp.ReadOnly {
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
		switch i.Name {
		case "capabilities/bounding-set":
			capList := strings.Join(strings.Split(i.Val, " "), ",")
			args = append(args, "--capability="+capList)
		}
	}

	return args, nil
}

// ContainerToNspawnArgs renders a prepared Container as a systemd-nspawn
// argument list ready to be executed
func (c *Container) ContainerToNspawnArgs() ([]string, error) {
	args := []string{
		"--uuid=" + c.Manifest.UUID.String(),
		"--directory=" + common.Stage1RootfsPath(c.Root),
	}

	for _, am := range c.Apps {
		a := c.Manifest.Apps.Get(am.Name)
		if a == nil {
			panic("could not find app in container manifest!")
		}
		aa, err := c.appToNspawnArgs(am, a.ImageID)
		if err != nil {
			return nil, fmt.Errorf("failed to construct args for app %q: %v", am.Name, err)
		}
		args = append(args, aa...)
	}

	return args, nil
}
