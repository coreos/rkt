package network

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path"
	"syscall"

	"github.com/coreos/rocket/network/util"
)

const RktNetPath = "/etc/rkt-net.conf.d"

type Net util.Net

func LoadNet(path string) (*Net, error) {
	c, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	n := &Net{Filename: path}
	if err = json.Unmarshal(c, n); err != nil {
		return nil, err
	}

	return n, nil
}

func LoadNets() ([]*Net, error) {
	dir, err := os.OpenFile(RktNetPath, syscall.O_RDONLY|syscall.O_DIRECTORY, 0)
	switch {
	case err == nil:
	case err.(*os.PathError).Err == syscall.ENOENT:
		return nil, nil
	default:
		return nil, err
	}
	defer dir.Close()

	dirents, err := dir.Readdir(0)
	if err != nil {
		return nil, err
	}

	var nets []*Net

	for _, dent := range dirents {
		if dent.IsDir() {
			continue
		}

		nf := path.Join(RktNetPath, dent.Name())
		n, err := LoadNet(nf)
		if err != nil {
			log.Printf("Error loading %v: %v", nf, err)
			continue
		}

		nets = append(nets, n)
	}

	return nets, nil
}
