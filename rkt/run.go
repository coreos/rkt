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
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/appc/spec/schema/types"
	"github.com/coreos/rkt/common"
	"github.com/coreos/rkt/pkg/label"
	"github.com/coreos/rkt/pkg/lock"
	pkgPod "github.com/coreos/rkt/pkg/pod"
	"github.com/coreos/rkt/pkg/user"
	"github.com/coreos/rkt/rkt/image"
	"github.com/coreos/rkt/stage0"
	"github.com/coreos/rkt/store/imagestore"
	"github.com/coreos/rkt/store/treestore"
	"github.com/hashicorp/errwrap"
	"github.com/spf13/cobra"
)

var (
	cmdRun = &cobra.Command{
		Use:   "run [--volume=name,kind=host,...] [--mount volume=VOL,target=PATH] IMAGE [-- image-args...[---]]...",
		Short: "Run image(s) in a pod in rkt",
		Long: `IMAGE should be a string referencing an image; either a hash, local file on
disk, or URL. They will be checked in that order and the first match will be
used.

Volumes are made available to the container via --volume. Mounts bind volumes
into each image's root within the container via --mount. --mount is
position-sensitive; occurring before any images applies to all images, occurring
after any images applies only to the nearest preceding image. Per-app mounts
take precedence over global ones if they have the same path.

An "--" may be used to inhibit rkt run's parsing of subsequent arguments, which
will instead be appended to the preceding image app's exec arguments. End the
image arguments with a lone "---" to resume argument parsing.`,
		Run: ensureSuperuser(runWrapper(runRun)),
	}
	flagPorts        portList
	flagNet          common.NetList
	flagPrivateUsers bool
	flagInheritEnv   bool
	flagExplicitEnv  envMap
	flagEnvFromFile  envFileMap
	flagInteractive  bool
	flagDNS          flagStringList
	flagDNSSearch    flagStringList
	flagDNSOpt       flagStringList
	flagNoOverlay    bool
	flagStoreOnly    bool
	flagNoStore      bool
	flagPodManifest  string
	flagMDSRegister  bool
	flagUUIDFileSave string
	flagHostname     string
)

func init() {
	cmdRkt.AddCommand(cmdRun)

	addStage1ImageFlags(cmdRun.Flags())
	cmdRun.Flags().Var(&flagPorts, "port", "ports to expose on the host (requires contained network). Syntax: --port=NAME:HOSTPORT")
	cmdRun.Flags().Var(&flagNet, "net", "configure the pod's networking. Optionally, pass a list of user-configured networks to load and set arguments to pass to each network, respectively. Syntax: --net[=n[:args], ...]")
	cmdRun.Flags().Lookup("net").NoOptDefVal = "default"
	cmdRun.Flags().BoolVar(&flagInheritEnv, "inherit-env", false, "inherit all environment variables not set by apps")
	cmdRun.Flags().BoolVar(&flagNoOverlay, "no-overlay", false, "disable overlay filesystem")
	cmdRun.Flags().BoolVar(&flagPrivateUsers, "private-users", false, "run within user namespaces.")
	cmdRun.Flags().Var(&flagExplicitEnv, "set-env", "environment variable to set for apps in the form name=value")
	cmdRun.Flags().Var(&flagEnvFromFile, "set-env-file", "path to an environment variables file")
	cmdRun.Flags().BoolVar(&flagInteractive, "interactive", false, "run pod interactively. If true, only one image may be supplied.")
	cmdRun.Flags().Var(&flagDNS, "dns", "name servers to write in /etc/resolv.conf")
	cmdRun.Flags().Var(&flagDNSSearch, "dns-search", "DNS search domains to write in /etc/resolv.conf")
	cmdRun.Flags().Var(&flagDNSOpt, "dns-opt", "DNS options to write in /etc/resolv.conf")
	cmdRun.Flags().BoolVar(&flagStoreOnly, "store-only", false, "use only available images in the store (do not discover or download from remote URLs)")
	cmdRun.Flags().BoolVar(&flagNoStore, "no-store", false, "fetch images ignoring the local store")
	cmdRun.Flags().StringVar(&flagPodManifest, "pod-manifest", "", "the path to the pod manifest. If it's non-empty, then only '--net', '--no-overlay' and '--interactive' will have effect")
	cmdRun.Flags().BoolVar(&flagMDSRegister, "mds-register", false, "register pod with metadata service. needs network connectivity to the host (--net=(default|default-restricted|host)")
	cmdRun.Flags().StringVar(&flagUUIDFileSave, "uuid-file-save", "", "write out pod UUID to specified file")
	cmdRun.Flags().StringVar(&flagHostname, "hostname", "", `pod's hostname. If empty, it will be "rkt-$PODUUID"`)
	cmdRun.Flags().Var((*appsVolume)(&rktApps), "volume", "volumes to make available in the pod")

	// per-app flags
	cmdRun.Flags().Var((*appAsc)(&rktApps), "signature", "local signature file to use in validating the preceding image")
	cmdRun.Flags().Var((*appExec)(&rktApps), "exec", "override the exec command for the preceding image")
	cmdRun.Flags().Var((*appMount)(&rktApps), "mount", "mount point binding a volume to a path within an app")
	cmdRun.Flags().Var((*appMemoryLimit)(&rktApps), "memory", "memory limit for the preceding image (example: '--memory=16Mi', '--memory=50M', '--memory=1G')")
	cmdRun.Flags().Var((*appCPULimit)(&rktApps), "cpu", "cpu limit for the preceding image (example: '--cpu=500m')")
	cmdRun.Flags().Var((*appUser)(&rktApps), "user", "user override for the preceding image (example: '--user=user')")
	cmdRun.Flags().Var((*appGroup)(&rktApps), "group", "group override for the preceding image (example: '--group=group')")
	cmdRun.Flags().Var((*appCapsRetain)(&rktApps), "caps-retain", "capability to retain (example: '--caps-retain=CAP_SYS_ADMIN')")
	cmdRun.Flags().Var((*appCapsRemove)(&rktApps), "caps-remove", "capability to remove (example: '--caps-remove=CAP_MKNOD')")
	cmdRun.Flags().Var((*appSeccompFilter)(&rktApps), "seccomp", "seccomp filter override (example: '--seccomp mode=retain,errno=EPERM,chmod,chown')")

	// For backwards compatibility
	cmdRun.Flags().Var((*appCapsRetain)(&rktApps), "cap-retain", "capability to retain (example: '--caps-retain=CAP_SYS_ADMIN')")
	cmdRun.Flags().Var((*appCapsRemove)(&rktApps), "cap-remove", "capability to remove (example: '--caps-remove=CAP_MKNOD')")
	cmdRun.Flags().MarkDeprecated("cap-retain", "use --caps-retain instead")
	cmdRun.Flags().MarkDeprecated("cap-remove", "use --caps-remove instead")

	flagPorts = portList{}
	flagDNS = flagStringList{}
	flagDNSSearch = flagStringList{}
	flagDNSOpt = flagStringList{}

	// Disable interspersed flags to stop parsing after the first non flag
	// argument. All the subsequent parsing will be done by parseApps.
	// This is needed to correctly handle image args
	cmdRun.Flags().SetInterspersed(false)
}

func runRun(cmd *cobra.Command, args []string) (exit int) {
	privateUsers := user.NewBlankUidRange()
	err := parseApps(&rktApps, args, cmd.Flags(), true)
	if err != nil {
		stderr.PrintE("error parsing app image arguments", err)
		return 1
	}

	if flagStoreOnly && flagNoStore {
		stderr.Print("both --store-only and --no-store specified")
		return 1
	}

	if flagPrivateUsers {
		if !common.SupportsUserNS() {
			stderr.Print("--private-users is not supported, kernel compiled without user namespace support")
			return 1
		}
		privateUsers.SetRandomUidRange(user.DefaultRangeCount)
	}

	if len(flagPorts) > 0 && flagNet.None() {
		stderr.Print("--port flag does not work with 'none' networking")
		return 1
	}
	if len(flagPorts) > 0 && flagNet.Host() {
		stderr.Print("--port flag does not work with 'host' networking")
		return 1
	}

	if flagMDSRegister && flagNet.None() {
		stderr.Print("--mds-register flag does not work with --net=none. Please use 'host', 'default' or an equivalent network")
		return 1
	}

	if len(flagPodManifest) > 0 && (len(flagPorts) > 0 || rktApps.Count() > 0 || flagStoreOnly || flagNoStore ||
		flagInheritEnv || !flagExplicitEnv.IsEmpty() || !flagEnvFromFile.IsEmpty() ||
		(*appsVolume)(&rktApps).String() != "" || (*appMount)(&rktApps).String() != "" || (*appExec)(&rktApps).String() != "" ||
		(*appUser)(&rktApps).String() != "" || (*appGroup)(&rktApps).String() != "" ||
		(*appCapsRetain)(&rktApps).String() != "" || (*appCapsRemove)(&rktApps).String() != "") {
		stderr.Print("conflicting flags set with --pod-manifest (see --help)")
		return 1
	}

	if flagInteractive && rktApps.Count() > 1 {
		stderr.Print("interactive option only supports one image")
		return 1
	}

	if rktApps.Count() < 1 && len(flagPodManifest) == 0 {
		stderr.Print("must provide at least one image or specify the pod manifest")
		return 1
	}

	s, err := imagestore.NewStore(storeDir())
	if err != nil {
		stderr.PrintE("cannot open store", err)
		return 1
	}

	ts, err := treestore.NewStore(treeStoreDir(), s)
	if err != nil {
		stderr.PrintE("cannot open treestore", err)
		return 1
	}

	config, err := getConfig()
	if err != nil {
		stderr.PrintE("cannot get configuration", err)
		return 1
	}

	s1img, err := getStage1Hash(s, ts, config)
	if err != nil {
		stderr.Error(err)
		return 1
	}

	fn := &image.Finder{
		S:                  s,
		Ts:                 ts,
		Ks:                 getKeystore(),
		Headers:            config.AuthPerHost,
		DockerAuth:         config.DockerCredentialsPerRegistry,
		InsecureFlags:      globalFlags.InsecureFlags,
		Debug:              globalFlags.Debug,
		TrustKeysFromHTTPS: globalFlags.TrustKeysFromHTTPS,

		StoreOnly: flagStoreOnly,
		NoStore:   flagNoStore,
		WithDeps:  true,
	}
	if err := fn.FindImages(&rktApps); err != nil {
		stderr.Error(err)
		return 1
	}

	p, err := pkgPod.NewPod(getDataDir())
	if err != nil {
		stderr.PrintE("error creating new pod", err)
		return 1
	}

	// if requested, write out pod UUID early so "rkt rm" can
	// clean it up even if something goes wrong
	if flagUUIDFileSave != "" {
		if err := pkgPod.WriteUUIDToFile(p.UUID, flagUUIDFileSave); err != nil {
			stderr.PrintE("error saving pod UUID to file", err)
			return 1
		}
	}

	processLabel, mountLabel, err := label.InitLabels("/var/run/rkt/mcs", []string{})
	if err != nil {
		stderr.PrintE("error initialising SELinux", err)
		return 1
	}

	p.MountLabel = mountLabel

	cfg := stage0.CommonConfig{
		MountLabel:   mountLabel,
		ProcessLabel: processLabel,
		Store:        s,
		TreeStore:    ts,
		Stage1Image:  *s1img,
		UUID:         p.UUID,
		Debug:        globalFlags.Debug,
	}

	ovlOk := true
	if err := common.PathSupportsOverlay(getDataDir()); err != nil {
		if oerr, ok := err.(common.ErrOverlayUnsupported); ok {
			stderr.Printf("disabling overlay support: %q", oerr.Error())
			ovlOk = false
		} else {
			stderr.PrintE("error determining overlay support", err)
			return 1
		}
	}

	useOverlay := !flagNoOverlay && ovlOk

	pcfg := stage0.PrepareConfig{
		CommonConfig:       &cfg,
		UseOverlay:         useOverlay,
		PrivateUsers:       privateUsers,
		SkipTreeStoreCheck: globalFlags.InsecureFlags.SkipOnDiskCheck(),
	}

	if len(flagPodManifest) > 0 {
		pcfg.PodManifest = flagPodManifest
	} else {
		pcfg.Ports = []types.ExposedPort(flagPorts)
		pcfg.InheritEnv = flagInheritEnv
		pcfg.ExplicitEnv = flagExplicitEnv.Strings()
		pcfg.EnvFromFile = flagEnvFromFile.Strings()
		pcfg.Apps = &rktApps
	}

	if globalFlags.Debug {
		stage0.InitDebug()
	}

	keyLock, err := lock.SharedKeyLock(lockDir(), common.PrepareLock)
	if err != nil {
		stderr.PrintE("cannot get shared prepare lock", err)
		return 1
	}
	err = stage0.Prepare(pcfg, p.Path(), p.UUID)
	if err != nil {
		stderr.PrintE("error setting up stage0", err)
		keyLock.Close()
		return 1
	}
	keyLock.Close()

	// get the lock fd for run
	lfd, err := p.Fd()
	if err != nil {
		stderr.PrintE("error getting pod lock fd", err)
		return 1
	}

	// skip prepared by jumping directly to run, we own this pod
	if err := p.ToRun(); err != nil {
		stderr.PrintE("unable to transition to run", err)
		return 1
	}

	rktgid, err := common.LookupGid(common.RktGroup)
	if err != nil {
		stderr.Printf("group %q not found, will use default gid when rendering images", common.RktGroup)
		rktgid = -1
	}

	rcfg := stage0.RunConfig{
		CommonConfig:         &cfg,
		Net:                  flagNet,
		LockFd:               lfd,
		Interactive:          flagInteractive,
		DNS:                  flagDNS,
		DNSSearch:            flagDNSSearch,
		DNSOpt:               flagDNSOpt,
		MDSRegister:          flagMDSRegister,
		LocalConfig:          globalFlags.LocalConfigDir,
		RktGid:               rktgid,
		Hostname:             flagHostname,
		InsecureCapabilities: globalFlags.InsecureFlags.SkipCapabilities(),
		InsecurePaths:        globalFlags.InsecureFlags.SkipPaths(),
		InsecureSeccomp:      globalFlags.InsecureFlags.SkipSeccomp(),
		UseOverlay:           useOverlay,
	}

	_, manifest, err := p.PodManifest()
	if err != nil {
		stderr.PrintE("cannot get the pod manifest", err)
		return 1
	}
	rcfg.Apps = manifest.Apps
	stage0.Run(rcfg, p.Path(), getDataDir()) // execs, never returns

	return 1
}

// portList implements the flag.Value interface to contain a set of mappings
// from port name --> host port
type portList []types.ExposedPort

func (pl *portList) Set(s string) error {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("%q is not in name:port format", s)
	}

	name, err := types.NewACName(parts[0])
	if err != nil {
		return errwrap.Wrap(fmt.Errorf("%q is not a valid port name", parts[0]), err)
	}

	port, err := strconv.ParseUint(parts[1], 10, 16)
	if err != nil {
		return fmt.Errorf("%q is not a valid port number", parts[1])
	}

	p := types.ExposedPort{
		Name:     *name,
		HostPort: uint(port),
	}

	*pl = append(*pl, p)
	return nil
}

func (pl *portList) String() string {
	var ps []string
	for _, p := range []types.ExposedPort(*pl) {
		ps = append(ps, fmt.Sprintf("%v:%v", p.Name, p.HostPort))
	}
	return strings.Join(ps, " ")
}

func (pl *portList) Type() string {
	return "portList"
}

// flagStringList implements the flag.Value interface to contain a set of strings
type flagStringList []string

func (dns *flagStringList) Set(s string) error {
	*dns = append(*dns, s)
	return nil
}

func (dns *flagStringList) String() string {
	return strings.Join(*dns, " ")
}

func (dns *flagStringList) Type() string {
	return "flagStringList"
}

// envMap implements the flag.Value interface to contain a set of name=value mappings
type envMap struct {
	mapping map[string]string
}

func (e *envMap) Set(s string) error {
	if e.mapping == nil {
		e.mapping = make(map[string]string)
	}
	pair := strings.SplitN(s, "=", 2)
	if len(pair) != 2 {
		return fmt.Errorf("environment variable must be specified as name=value")
	}
	if _, exists := e.mapping[pair[0]]; exists {
		return fmt.Errorf("environment variable %q already set", pair[0])
	}
	e.mapping[pair[0]] = pair[1]
	return nil
}

func (e *envMap) IsEmpty() bool {
	return len(e.mapping) == 0
}

func (e *envMap) String() string {
	return strings.Join(e.Strings(), "\n")
}

func (e *envMap) Strings() []string {
	var env []string
	for n, v := range e.mapping {
		env = append(env, n+"="+v)
	}
	return env
}

func (e *envMap) Type() string {
	return "envMap"
}

// envFileMap
type envFileMap struct {
	mapping map[string]string
}

func commentLine(s string) bool {
	return strings.HasPrefix(s, "#") || strings.HasPrefix(s, ";")
}

func (e *envFileMap) Set(s string) error {
	if e.mapping == nil {
		e.mapping = make(map[string]string)
	}
	file, err := os.Open(s)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// Skip empty lines
		if len(line) == 0 {
			continue
		}
		// Skip comments
		if commentLine(line) {
			continue
		}
		pair := strings.SplitN(line, "=", 2)
		// Malformed lines
		if len(pair) != 2 || len(pair[0]) == 0 {
			return fmt.Errorf("environment variable must be specified as name=value (file %q)", file)
		}
		if _, exists := e.mapping[pair[0]]; exists {
			return fmt.Errorf("environment variable %q already set (file %q)", pair[0], file)
		}
		e.mapping[pair[0]] = pair[1]
	}
	return nil
}

func (e *envFileMap) IsEmpty() bool {
	return len(e.mapping) == 0
}

func (e *envFileMap) String() string {
	return strings.Join(e.Strings(), "\n")
}

func (e *envFileMap) Strings() []string {
	var env []string
	for n, v := range e.mapping {
		env = append(env, n+"="+v)
	}
	return env
}

func (e *envFileMap) Type() string {
	return "envFileMap"
}
