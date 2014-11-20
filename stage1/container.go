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

	"github.com/containers/standard/schema"
	"github.com/containers/standard/schema/types"
	"github.com/coreos-inc/rkt/rkt"
	"github.com/coreos/go-systemd/unit"
)

// Container encapsulates a ContainerRuntimeManifest and AppManifests
type Container struct {
	Root     string // root directory where the container will be located
	Manifest *schema.ContainerRuntimeManifest
	Apps     map[string]*schema.AppManifest
}

// LoadContainer loads a Container Runtime Manifest (as prepared by stage0) and
// its associated Application Manifests, under $root/stage1/opt/stage1/$apphash
func LoadContainer(root string) (*Container, error) {
	c := &Container{
		Root: root,
		Apps: make(map[string]*schema.AppManifest),
	}

	buf, err := ioutil.ReadFile(rkt.ContainerManifestPath(c.Root))
	if err != nil {
		return nil, fmt.Errorf("failed reading container runtime manifest: %v", err)
	}

	cm := &schema.ContainerRuntimeManifest{}
	if err := json.Unmarshal(buf, cm); err != nil {
		return nil, fmt.Errorf("failed unmarshalling container runtime manifest: %v", err)
	}
	c.Manifest = cm

	for _, app := range c.Manifest.Apps {
		ampath := rkt.AppManifestPath(c.Root, app.ImageID.String())
		buf, err := ioutil.ReadFile(ampath)
		if err != nil {
			return nil, fmt.Errorf("failed reading app manifest %q: %v", ampath, err)
		}

		am := &schema.AppManifest{}
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

// appToSystemd transforms the provided app manifest into a systemd service unit
func (c *Container) appToSystemd(am *schema.AppManifest, id types.Hash) error {
	name := am.Name.String()
	execStart := strings.Join(am.Exec, " ")
	opts := []*unit.UnitOption{
		&unit.UnitOption{"Unit", "Description", name},
		&unit.UnitOption{"Unit", "DefaultDependencies", "false"},
		&unit.UnitOption{"Service", "Restart", "no"},
		&unit.UnitOption{"Service", "RootDirectory", rkt.RelAppRootfsPath(id.String())},
		&unit.UnitOption{"Service", "ExecStart", execStart},
		&unit.UnitOption{"Service", "User", am.User},
		&unit.UnitOption{"Service", "Group", am.Group},
		&unit.UnitOption{"Service", "Type", "oneshot"},
		&unit.UnitOption{"Unit", "Requires", "shutdown.service"},
		&unit.UnitOption{"Unit", "Before", "shutdown.service"},
	}

	for _, eh := range am.EventHandlers {
		var typ string
		switch eh.Event {
		case "preStart":
			typ = "ExecStartPre"
		case "postStop":
			typ = "ExecStopPost"
		default:
			return fmt.Errorf("unrecognized eventHandler: %v", eh.Event)
		}
		exec := strings.Join(eh.Exec, " ")
		opts = append(opts, &unit.UnitOption{"Service", typ, exec})
	}

	env := am.Environment
	env["AC_APP_NAME"] = string(am.Name)
	for ek, ev := range env {
		ee := fmt.Sprintf(`"%s=%s"`, ek, ev)
		opts = append(opts, &unit.UnitOption{"Service", "Environment", ee})
	}

	file, err := os.OpenFile(rkt.ServiceFilePath(c.Root, name), os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to create service file: %v", err)
	}
	defer file.Close()

	_, err = io.Copy(file, unit.Serialize(opts))
	if err != nil {
		return fmt.Errorf("failed to write service file: %v", err)
	}

	if err = os.Symlink(path.Join("..", rkt.ServiceName(name)), rkt.WantLinkPath(c.Root, name)); err != nil {
		return fmt.Errorf("failed to link service want: %v", err)
	}

	return nil
}

// ContainerToSystemd creates the appropriate systemd service unit files for
// all the constituent apps of the Container
func (c *Container) ContainerToSystemd() error {
	if err := os.MkdirAll(rkt.ServicesPath(c.Root), 0640); err != nil {
		return fmt.Errorf("failed to create services directory: %v", err)
	}
	if err := os.MkdirAll(rkt.WantsPath(c.Root), 0640); err != nil {
		return fmt.Errorf("failed to create wants directory: %v", err)
	}
	for _, am := range c.Apps {
		id := c.Manifest.Apps[am.Name].ImageID
		if err := c.appToSystemd(am, id); err != nil {
			return fmt.Errorf("failed to transform app %q into systemd service: %v", am.Name, err)
		}
	}

	return nil
}

// appToNspawnArgs transforms the given app manifest, with the given associated
// app image id, into a subset of applicable systemd-nspawn argument
func (c *Container) appToNspawnArgs(am *schema.AppManifest, id types.Hash) ([]string, error) {
	args := []string{}
	name := am.Name.String()

	vols := make(map[types.ACLabel]types.Volume)
	for _, v := range c.Manifest.Volumes {
		for _, f := range v.Fulfills {
			vols[f] = v
		}
	}

	for key, mp := range am.MountPoints {
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
		opt[3] = filepath.Join(rkt.RelAppRootfsPath(id.String()), mp.Path)

		args = append(args, strings.Join(opt, ""))
	}

	return args, nil
}

// ContainerToNspawnArgs renders a prepared Container as a systemd-nspawn
// argument list ready to be executed
func (c *Container) ContainerToNspawnArgs() ([]string, error) {
	args := []string{
		"--uuid=" + c.Manifest.UUID.String(),
		"--directory=" + rkt.Stage1RootfsPath(c.Root),
	}

	for _, am := range c.Apps {
		id := c.Manifest.Apps[am.Name].ImageID
		aa, err := c.appToNspawnArgs(am, id)
		if err != nil {
			return nil, fmt.Errorf("failed to construct args for app %q: %v", am.Name, err)
		}
		args = append(args, aa...)
	}

	return args, nil
}
