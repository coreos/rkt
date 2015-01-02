package network

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

type NetPlugin struct {
	Name     string `json:"name,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
	Command  struct {
		Add []string `json:"add,omitempty"`
		Del []string `json:"del,omitempty"`
	}
}

const RktNetPluginsPath = "/etc/rkt-net-plugins.conf.d"

func LoadNetPlugin(path string) (*NetPlugin, error) {
	c, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	np := &NetPlugin{}
	if err = json.Unmarshal(c, np); err != nil {
		return nil, err
	}

	return np, nil
}

func LoadNetPlugins() (map[string]*NetPlugin, error) {
	plugins := make(map[string]*NetPlugin)

	dir, err := os.OpenFile(RktNetPluginsPath, syscall.O_RDONLY|syscall.O_DIRECTORY, 0)
	switch {
	case err == nil:
	case err.(*os.PathError).Err == syscall.ENOENT:
		return plugins, nil
	default:
		return nil, err
	}
	defer dir.Close()

	dents, err := dir.Readdir(0)
	if err != nil {
		return nil, err
	}

	for _, dent := range dents {
		if dent.IsDir() {
			continue
		}

		npPath := filepath.Join(RktNetPluginsPath, dent.Name())
		np, err := LoadNetPlugin(npPath)
		if err != nil {
			log.Printf("Loading %v: %v", npPath, err)
			continue
		}

		plugins[np.Name] = np
	}

	return plugins, nil
}

func (np *NetPlugin) Add(n *Net, contID, netns, confFile, args, ifName string) error {
	fmt.Println("NetPlugin.Add:", contID, netns, confFile, args, ifName)

	switch {
	case np.Endpoint != "":
		return execHTTP(np.Endpoint, "add", n.Name, contID, netns, confFile, args, ifName)

	default:
		if len(np.Command.Add) == 0 {
			return fmt.Errorf("plugin does not define command.add")
		}

		return execCmd(np.Command.Add, n.Name, contID, netns, confFile, args, ifName)
	}
}

func (np *NetPlugin) Del(n *Net, contID, netns, confFile, args, ifName string) error {
	switch {
	case np.Endpoint != "":
		return execHTTP(np.Endpoint, "del", n.Name, contID, netns, confFile, args, ifName)

	default:
		if len(np.Command.Del) == 0 {
			return fmt.Errorf("plugin does not define command.del")
		}

		return execCmd(np.Command.Del, n.Name, contID, netns, confFile, args, ifName)
	}
}

func execHTTP(ep, cmd, netName, contID, netns, confFile, args, ifName string) error {
	return fmt.Errorf("not implemented")
}

func replaceAll(xs []string, what, with string) {
	for i, x := range xs {
		xs[i] = strings.Replace(x, what, with, -1)
	}
}

func execCmd(cmd []string, netName, contID, netns, confFile, args, ifName string) error {
	replaceAll(cmd, "{net-name}", netName)
	replaceAll(cmd, "{cont-id}", contID)
	replaceAll(cmd, "{netns}", netns)
	replaceAll(cmd, "{conf-file}", confFile)
	replaceAll(cmd, "{args}", args)
	replaceAll(cmd, "{if-name}", ifName)

	c := exec.Command(cmd[0], cmd[1:]...)
	//c.Stdout = &bytes.Buffer{}
	c.Stderr = os.Stderr
	return c.Run()
}
