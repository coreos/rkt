// Copyright 2017 Aporeto Inc
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
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"

	rktlog "github.com/rkt/rkt/pkg/log"

	"github.com/appc/spec/schema/types"
	stage1commontypes "github.com/rkt/rkt/stage1/common/types"
)

var (
	interactive bool
	ip          string
	gw          string
	route       string
	bridge      string
	uuid        string
	u, _        = user.Current()
	log         *rktlog.Logger
	diag        *rktlog.Logger
)

func init() {
	flag.BoolVar(&interactive, "interactive", false, "iteractive")
	flag.StringVar(&ip, "ip", "", "ip")
	flag.StringVar(&gw, "gw", "", "gateway")
	flag.StringVar(&route, "route", "", "route")
	flag.StringVar(&bridge, "bridge", "", "bridge")

	log, diag, _ = rktlog.NewLogSet("xen", false)
}

func main() {
	flag.Parse()
	l := len(flag.Args())
	if l > 0 {
		uuid = flag.Args()[l-1]
	}

	uuidt, err := types.NewUUID(uuid)
	if err != nil {
		log.FatalE("UUID is missing or malformed", err)
		os.Exit(254)
	}
	root := "."
	rp := stage1commontypes.RuntimePod{}
	p, err := stage1commontypes.LoadPod(root, uuidt, &rp)
	if err != nil {
		log.FatalE("failed to load pod", err)
		os.Exit(254)
	}

	// Only support 1 app for now
	app := p.Manifest.Apps[0]

	workpath, _ := filepath.Abs(filepath.Dir("."))
	f, err := os.Create(fmt.Sprintf("%s/stage1/rootfs/vm.0", workpath))
	if err != nil {
		log.FatalE("failed to create VM config file", err)
		os.Exit(254)
	}

	f.WriteString(fmt.Sprintf("kernel='%s/stage1/rootfs/bzImage'\n", workpath))
	f.WriteString(fmt.Sprintf("ramdisk='%s/stage1/rootfs/initrd'\n", workpath))
	f.WriteString("memory = 1024\n")
	f.WriteString("vcpus = 2\n")
	f.WriteString("serial='pty'\n")
	f.WriteString("boot='c'\n")
	f.WriteString("vfb=['vnc=1']\n")
	if bridge == "vif" {
		f.WriteString(fmt.Sprintf("vif=['script=vif-nat,ip=%s']\n", ip))
	} else {
		f.WriteString(fmt.Sprintf("vif=['bridge=%s']\n", bridge))
	}
	f.WriteString(fmt.Sprintf("xen_9pfs=['tag=share_dir,security_model=none,path=%s/stage1/rootfs/opt/stage2/%s' ]\n", workpath, app.Name))
	f.WriteString(fmt.Sprintf("extra='console=hvc0 root=9p ip=%s gw=%s route=%s'\n", ip, gw, route))
	f.WriteString(fmt.Sprintf("name=\"%s\"\n", uuid))
	f.Close()

	f, err = os.Create(fmt.Sprintf("%s/stage1/rootfs/opt/stage2/%s/cmdline", workpath, app.Name))
	if err != nil {
		log.FatalE("failed to create VM config file", err)
		os.Exit(254)
	}
	f.WriteString(fmt.Sprintf("\"%s\"\n", strings.Join(app.App.Exec, "\" \"")))
	f.Close()

	env := os.Environ()
	args := []string{"xl", "create"}
	if interactive {
		args = append(args, "-c")
	}
	args = append(args, fmt.Sprintf("%s/stage1/rootfs/vm.0", workpath))
	abspath, err := exec.LookPath("xl")
	if err != nil {
		log.FatalE("cannot find xl in $PATH", err)
		os.Exit(254)
	}

	err = syscall.Exec(abspath, args, env)
	if err != nil {
		log.FatalE("failed to create VM", err)
		os.Exit(254)
	}
	os.Exit(0)
}
