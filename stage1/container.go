//+build linux

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/coreos/rocket/Godeps/_workspace/src/github.com/coreos/go-systemd/unit"
	"github.com/coreos/rocket/app-container/schema"
	"github.com/coreos/rocket/app-container/schema/types"
	rktpath "github.com/coreos/rocket/path"
)

// Container encapsulates a ContainerRuntimeManifest and ImageManifests
type Container struct {
	Root     string // root directory where the container will be located
	Manifest *schema.ContainerRuntimeManifest
	Apps     map[string]*schema.ImageManifest
}

// LoadContainer loads a Container Runtime Manifest (as prepared by stage0) and
// its associated Application Manifests, under $root/stage1/opt/stage1/$apphash
func LoadContainer(root string) (*Container, error) {
	c := &Container{
		Root: root,
		Apps: make(map[string]*schema.ImageManifest),
	}

	buf, err := ioutil.ReadFile(rktpath.ContainerManifestPath(c.Root))
	if err != nil {
		return nil, fmt.Errorf("failed reading container runtime manifest: %v", err)
	}

	cm := &schema.ContainerRuntimeManifest{}
	if err := json.Unmarshal(buf, cm); err != nil {
		return nil, fmt.Errorf("failed unmarshalling container runtime manifest: %v", err)
	}
	c.Manifest = cm

	for _, app := range c.Manifest.Apps {
		ampath := rktpath.ImageManifestPath(c.Root, app.ImageID)
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

// appToSystemd transforms the provided app manifest into systemd units
func (c *Container) appToSystemd(am *schema.ImageManifest, id types.Hash) error {
	name := am.Name.String()
	app := am.App
	execStart := strings.Join(app.Exec, " ")
	opts := []*unit.UnitOption{
		&unit.UnitOption{"Unit", "Description", name},
		&unit.UnitOption{"Unit", "DefaultDependencies", "false"},
		&unit.UnitOption{"Unit", "OnFailureJobMode", "isolate"},
		&unit.UnitOption{"Unit", "OnFailure", "reaper.service"},
		&unit.UnitOption{"Unit", "Wants", "exit-watcher.service"},
		&unit.UnitOption{"Service", "Restart", "no"},
		&unit.UnitOption{"Service", "RootDirectory", rktpath.RelAppRootfsPath(id)},
		&unit.UnitOption{"Service", "ExecStart", execStart},
		&unit.UnitOption{"Service", "User", app.User},
		&unit.UnitOption{"Service", "Group", app.Group},
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
		exec := strings.Join(eh.Exec, " ")
		opts = append(opts, &unit.UnitOption{"Service", typ, exec})
	}

	env := app.Environment
	env["AC_APP_NAME"] = name
	for ek, ev := range env {
		ee := fmt.Sprintf(`"%s=%s"`, ek, ev)
		opts = append(opts, &unit.UnitOption{"Service", "Environment", ee})
	}

	saPorts := []types.Port{}
	for _, p := range app.Ports {
		if p.SocketActivated {
			saPorts = append(saPorts, p)
		}
	}

	if len(saPorts) > 0 {
		sockopts := []*unit.UnitOption{
			&unit.UnitOption{"Unit", "Description", name + " socket-activated ports"},
			&unit.UnitOption{"Unit", "DefaultDependencies", "false"},
			&unit.UnitOption{"Socket", "BindIPv6Only", "both"},
			&unit.UnitOption{"Socket", "Service", ServiceUnitName(id)},
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
			sockopts = append(sockopts, &unit.UnitOption{"Socket", proto, fmt.Sprintf("%v", sap.Port)})
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

		opts = append(opts, &unit.UnitOption{"Unit", "Requires", SocketUnitName(id)})
	}

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
	for _, v := range c.Manifest.Volumes {
		for _, f := range v.Fulfills {
			vols[f] = v
		}
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
		opt[3] = filepath.Join(rktpath.RelAppRootfsPath(id), mp.Path)

		args = append(args, strings.Join(opt, ""))
	}

	return args, nil
}

// ContainerToNspawnArgs renders a prepared Container as a systemd-nspawn
// argument list ready to be executed
func (c *Container) ContainerToNspawnArgs() ([]string, error) {
	args := []string{
		"--uuid=" + c.Manifest.UUID.String(),
		"--directory=" + rktpath.Stage1RootfsPath(c.Root),
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
