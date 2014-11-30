package main

// this implements /init of stage1/host_nspawn-systemd

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/coreos/rocket/metadata"
	"github.com/coreos/rocket/path"
)

const (
	// Path to systemd-nspawn binary within the stage1 rootfs
	nspawnBin = "/usr/bin/systemd-nspawn"
)

var (
	debug       bool
	metadataSvc string
	bridgeName  string
	bridgeCIDR  string
)

func init() {
	flag.BoolVar(&debug, "debug", false, "Run in debug mode")
	flag.StringVar(&metadataSvc, "metadata-svc", "", "Launch specified metadata svc")
	flag.StringVar(&bridgeName, "bridge", "rkt0", "Bridge name to create/connect to")
	flag.StringVar(&bridgeCIDR, "bridge-cidr", "10.111.0.1/16", "Bridge address")
}

func stage1() int {
	root := "."
	c, err := LoadContainer(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load container: %v\n", err)
		return 1
	}

	c.MetadataSvcURL = metadata.MetadataSvcPubURL()
	c.MachineName = fmt.Sprintf("%x", c.Manifest.UUID[:4])
	c.Bridge = bridgeName
	c.BridgeAddr = bridgeCIDR

	if err = c.ContainerToSystemd(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to configure systemd: %v\n", err)
		return 2
	}

	// TODO(philips): compile a static version of systemd-nspawn with this
	// stupidity patched out
	_, err = os.Stat("/run/systemd/system")
	if os.IsNotExist(err) {
		os.MkdirAll("/run/systemd/system", 0755)
	}

	ex := filepath.Join(path.Stage1RootfsPath(c.Root), nspawnBin)
	if _, err := os.Stat(ex); err != nil {
		fmt.Fprintf(os.Stderr, "Failed locating nspawn: %v\n", err)
		return 3
	}

	args := []string{
		"--boot",           // Launch systemd in the container
		"--register=false", // We cannot assume the host system is running a compatible systemd
	}

	if !debug {
		args = append(args, "--quiet") // silence most nspawn output (log_warning is currently not covered by this)
	}

	nsargs, err := c.ContainerToNspawnArgs()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate nspawn args: %v\n", err)
		return 4
	}
	args = append(args, nsargs...)

	// Arguments to systemd
	args = append(args, "--")
	args = append(args, "--default-standard-output=tty") // redirect all service logs straight to tty
	if !debug {
		args = append(args, "--log-target=null") // silence systemd output inside container
		args = append(args, "--show-status=0")   // silence systemd initialization status output
	}

	if metadataSvc != "" {
		if err = launchMetadataSvc(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to launch metadata svc: %v\n", err)
			return 5
		}
	}

	ip, err := setupNetwork(c)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to setup network: %v\n", err)
		return 6
	}
	defer teardownNetwork(c, ip)

	if err = registerContainer(c, ip); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to register container: %v\n", err)
		return 7
	}
	defer unregisterContainer(c)

	cmd := exec.Command(ex, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to execute nspawn: %v\n", err)
		return 8
	}

	return 0
}

func main() {
	flag.Parse()
	// move code into stage1() helper so defered fns get run
	os.Exit(stage1())
}

func launchMetadataSvc() error {
	fmt.Println("Launching metadatasvc: ", metadataSvc)

	// use socket activation protocol to avoid race-condition of
	// service becoming ready
	// TODO(eyakubovich): remove hard-coded port
	l, err := net.ListenTCP("tcp4", &net.TCPAddr{Port: metadata.MetadataSvcPrvPort})
	if err != nil {
		if err.(*net.OpError).Err.(*os.SyscallError).Err == syscall.EADDRINUSE {
			// assume metadatasvc is already running
			return nil
		}
		return err
	}

	defer l.Close()

	lf, err := l.File()
	if err != nil {
		return err
	}

	// parse metadataSvc into exe and args
	args := strings.Split(metadataSvc, " ")
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = append(os.Environ(), "LISTEN_FDS=1")
	cmd.ExtraFiles = []*os.File{lf}

	return cmd.Start()
}
