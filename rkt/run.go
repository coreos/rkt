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
	"fmt"
	"strconv"
	"strings"

	"github.com/appc/spec/schema/types"
	"github.com/coreos/rkt/common"
	"github.com/coreos/rkt/pkg/label"
	"github.com/coreos/rkt/pkg/lock"
	"github.com/coreos/rkt/pkg/uid"
	"github.com/coreos/rkt/rkt/image"
	"github.com/coreos/rkt/stage0"
	"github.com/coreos/rkt/store"
	"github.com/spf13/cobra"
)

var (
	cmdRun = &cobra.Command{
		Use:   "run [--volume=name,kind=host,...] [--mount volume=VOL,target=PATH] IMAGE [-- image-args...[---]]...",
		Short: "Run image(s) in a pod in rkt",
		Long: `IMAGE should be a string referencing an image; either a hash, local file on disk, or URL.
They will be checked in that order and the first match will be used.

Volumes are made available to the container via --volume.
Mounts bind volumes into each image's root within the container via --mount.
--mount is position-sensitive; occuring before any images applies to all images,
occuring after any images applies only to the nearest preceding image. Per-app
mounts take precedence over global ones if they have the same path.

An "--" may be used to inhibit rkt run's parsing of subsequent arguments,
which will instead be appended to the preceding image app's exec arguments.
End the image arguments with a lone "---" to resume argument parsing.`,
		Run: runWrapper(runRun),
	}
	flagPorts        portList
	flagNet          common.NetList
	flagPrivateUsers bool
	flagInheritEnv   bool
	flagExplicitEnv  envMap
	flagInteractive  bool
	flagDNS          string
	flagNoOverlay    bool
	flagStoreOnly    bool
	flagNoStore      bool
	flagPodManifest  string
	flagMDSRegister  bool
	flagUUIDFileSave string
)

func init() {
	cmdRkt.AddCommand(cmdRun)

	addStage1ImageFlag(cmdRun.Flags())
	cmdRun.Flags().Var(&flagPorts, "port", "ports to expose on the host (requires --net)")
	cmdRun.Flags().Var(&flagNet, "net", "configure the pod's networking and optionally pass a list of user-configured networks to load and arguments to pass to them. syntax: --net[=n[:args], ...]")
	cmdRun.Flags().Lookup("net").NoOptDefVal = "default"
	cmdRun.Flags().BoolVar(&flagInheritEnv, "inherit-env", false, "inherit all environment variables not set by apps")
	cmdRun.Flags().BoolVar(&flagNoOverlay, "no-overlay", false, "disable overlay filesystem")
	cmdRun.Flags().BoolVar(&flagPrivateUsers, "private-users", false, "Run within user namespaces (experimental).")
	cmdRun.Flags().Var(&flagExplicitEnv, "set-env", "an environment variable to set for apps in the form name=value")
	cmdRun.Flags().BoolVar(&flagInteractive, "interactive", false, "run pod interactively. If true, only one image may be supplied.")
	cmdRun.Flags().StringVar(&flagDNS, "dns", "", "comma separated list of name servers")
	cmdRun.Flags().BoolVar(&flagStoreOnly, "store-only", false, "use only available images in the store (do not discover or download from remote URLs)")
	cmdRun.Flags().BoolVar(&flagNoStore, "no-store", false, "fetch images ignoring the local store")
	cmdRun.Flags().StringVar(&flagPodManifest, "pod-manifest", "", "the path to the pod manifest. If it's non-empty, then only '--net', '--no-overlay' and '--interactive' will have effects")
	cmdRun.Flags().BoolVar(&flagMDSRegister, "mds-register", false, "register pod with metadata service. needs network connectivity to the host (--net=(default|default-restricted|host)")
	cmdRun.Flags().StringVar(&flagUUIDFileSave, "uuid-file-save", "", "write out pod UUID to specified file")
	cmdRun.Flags().Var((*appsVolume)(&rktApps), "volume", "volumes to make available in the pod")

	// per-app flags
	cmdRun.Flags().Var((*appAsc)(&rktApps), "signature", "local signature file to use in validating the preceding image")
	cmdRun.Flags().Var((*appExec)(&rktApps), "exec", "override the exec command for the preceding image")
	cmdRun.Flags().Var((*appMount)(&rktApps), "mount", "mount point binding a volume to a path within an app")
	cmdRun.Flags().Var((*appMemoryLimit)(&rktApps), "memory", "memory limit for the preceding image (example: '--memory=16Mi', '--memory=50M', '--memory=1G')")
	cmdRun.Flags().Var((*appCPULimit)(&rktApps), "cpu", "cpu limit for the preceding image (example: '--cpu=500m')")

	flagPorts = portList{}

	// Disable interspersed flags to stop parsing after the first non flag
	// argument. All the subsequent parsing will be done by parseApps.
	// This is needed to correctly handle image args
	cmdRun.Flags().SetInterspersed(false)
}

func runRun(cmd *cobra.Command, args []string) (exit int) {
	privateUsers := uid.NewBlankUidRange()
	err := parseApps(&rktApps, args, cmd.Flags(), true)
	if err != nil {
		stderr("run: error parsing app image arguments: %v", err)
		return 1
	}

	if flagStoreOnly && flagNoStore {
		stderr("both --store-only and --no-store specified")
		return 1
	}

	if flagPrivateUsers {
		if !common.SupportsUserNS() {
			stderr("run: --private-users is not supported, kernel compiled without user namespace support")
			return 1
		}
		privateUsers.SetRandomUidRange(uid.DefaultRangeCount)
	}

	if len(flagPorts) > 0 && flagNet.None() {
		stderr("--port flag does not work with 'none' networking")
		return 1
	}
	if len(flagPorts) > 0 && flagNet.Host() {
		stderr("--port flag does not work with 'host' networking")
		return 1
	}

	if flagMDSRegister && flagNet.None() {
		stderr("--mds-register flag does not work with --net=none. Please use 'host', 'default' or an equivalent network")
		return 1
	}

	if len(flagPodManifest) > 0 && (len(flagPorts) > 0 || flagInheritEnv || !flagExplicitEnv.IsEmpty() || rktApps.Count() > 0 || flagStoreOnly || flagNoStore) {
		stderr("conflicting flags set with --pod-manifest (see --help)")
		return 1
	}

	if flagInteractive && rktApps.Count() > 1 {
		stderr("run: interactive option only supports one image")
		return 1
	}

	if rktApps.Count() < 1 && len(flagPodManifest) == 0 {
		stderr("run: must provide at least one image or specify the pod manifest")
		return 1
	}

	s, err := store.NewStore(getDataDir())
	if err != nil {
		stderr("run: cannot open store: %v", err)
		return 1
	}

	config, err := getConfig()
	if err != nil {
		stderr("run: cannot get configuration: %v", err)
		return 1
	}

	s1img, err := getStage1Hash(s, cmd)
	if err != nil {
		stderr("run: %v", err)
		return 1
	}

	fn := &image.Finder{
		S:                  s,
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
		stderr("run: %v", err)
		return 1
	}

	p, err := newPod()
	if err != nil {
		stderr("Error creating new pod: %v", err)
		return 1
	}

	// if requested, write out pod UUID early so "rkt rm" can
	// clean it up even if something goes wrong
	if flagUUIDFileSave != "" {
		if err := writeUUIDToFile(p.uuid, flagUUIDFileSave); err != nil {
			stderr("Error saving pod UUID to file: %v", err)
			return 1
		}
	}

	processLabel, mountLabel, err := label.InitLabels([]string{"mcsdir:/var/run/rkt/mcs"})
	if err != nil {
		stderr("Error initialising SELinux: %v", err)
		return 1
	}

	cfg := stage0.CommonConfig{
		MountLabel:   mountLabel,
		ProcessLabel: processLabel,
		Store:        s,
		Stage1Image:  *s1img,
		UUID:         p.uuid,
		Debug:        globalFlags.Debug,
	}

	pcfg := stage0.PrepareConfig{
		CommonConfig:       &cfg,
		UseOverlay:         !flagNoOverlay && common.SupportsOverlay(),
		PrivateUsers:       privateUsers,
		SkipTreeStoreCheck: globalFlags.InsecureFlags.SkipOnDiskCheck(),
	}

	if len(flagPodManifest) > 0 {
		pcfg.PodManifest = flagPodManifest
	} else {
		pcfg.Ports = []types.ExposedPort(flagPorts)
		pcfg.InheritEnv = flagInheritEnv
		pcfg.ExplicitEnv = flagExplicitEnv.Strings()
		pcfg.Apps = &rktApps
	}

	if globalFlags.Debug {
		stage0.InitDebug()
	}

	keyLock, err := lock.SharedKeyLock(lockDir(), common.PrepareLock)
	if err != nil {
		stderr("rkt: cannot get shared prepare lock: %v", err)
		return 1
	}
	err = stage0.Prepare(pcfg, p.path(), p.uuid)
	if err != nil {
		stderr("run: error setting up stage0: %v", err)
		keyLock.Close()
		return 1
	}
	keyLock.Close()

	// get the lock fd for run
	lfd, err := p.Fd()
	if err != nil {
		stderr("Error getting pod lock fd: %v", err)
		return 1
	}

	// skip prepared by jumping directly to run, we own this pod
	if err := p.xToRun(); err != nil {
		stderr("run: unable to transition to run: %v", err)
		return 1
	}

	rktgid, err := common.LookupGid(common.RktGroup)
	if err != nil {
		stderr("run: group %q not found, will use default gid when rendering images", common.RktGroup)
		rktgid = -1
	}

	rcfg := stage0.RunConfig{
		CommonConfig: &cfg,
		Net:          flagNet,
		LockFd:       lfd,
		Interactive:  flagInteractive,
		DNS:          strings.Split(flagDNS, ","),
		MDSRegister:  flagMDSRegister,
		LocalConfig:  globalFlags.LocalConfigDir,
		RktGid:       rktgid,
	}

	apps, err := p.getApps()
	if err != nil {
		stderr("run: cannot get the appList in the pod manifest: %v", err)
		return 1
	}
	rcfg.Apps = apps
	stage0.Run(rcfg, p.path(), getDataDir()) // execs, never returns

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
		return fmt.Errorf("%q is not a valid port name: %v", parts[0], err)
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
