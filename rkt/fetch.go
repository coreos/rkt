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

package main

import (
	"fmt"
	"net/rpc"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/appc/spec/schema/types"
	"github.com/coreos/rkt/common/apps"
	"github.com/coreos/rkt/store"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/spf13/cobra"
)

const (
	defaultOS   = runtime.GOOS
	defaultArch = runtime.GOARCH
)

var (
	cmdFetch = &cobra.Command{
		Use:   "fetch IMAGE_URL...",
		Short: "Fetch image(s) and store them in the local store",
		Run:   runWrapper(runFetch),
	}
	flagP2pDuration int
)

func init() {
	cmdRkt.AddCommand(cmdFetch)
	// Disable interspersed flags to stop parsing after the first non flag
	// argument. All the subsequent parsing will be done by parseApps.
	// This is needed to correctly handle multiple IMAGE --signature=sigfile options
	cmdFetch.Flags().SetInterspersed(false)

	cmdFetch.Flags().Var((*appAsc)(&rktApps), "signature", "local signature file to use in validating the preceding image")
	cmdFetch.Flags().BoolVar(&flagStoreOnly, "store-only", false, "use only available images in the store (do not discover or download from remote URLs)")
	cmdFetch.Flags().BoolVar(&flagNoStore, "no-store", false, "fetch images ignoring the local store")
	cmdFetch.Flags().IntVar(&flagP2pDuration, "p2p-duration", 10, "p2p continue service duration (minutes) after download finished")
}

func p2pFetch(args []string) int {
	duration := strconv.Itoa(flagP2pDuration)
	//start p2p client
	cmd := exec.Command("./torrent", args[0], duration)
	if err := cmd.Start(); err != nil {
		stderr("start p2p process err: %v", err)
		return 1
	}

	//wait for p2p client start
	time.Sleep(time.Second * time.Duration(7))

	//connet to p2p client process and get download rate
	client, err := rpc.DialHTTP("tcp", "127.0.0.1:1234")
	if err != nil {
		stderr("connet to p2p client err: %v", err)
		return 1
	}
	reply := make([]string, 1)
	if err := client.Call("Download.GetTotalSize", struct{}{}, &reply); err != nil {
		stderr("get download total size err: %v", err)
		return 1
	}
	totalSize := reply[0]

	if err := client.Call("Download.GetFile", struct{}{}, &reply); err != nil {
		stderr("get download file err: %v", err)
		return 1
	}
	aciImage := reply[0]

	//get rate for download
	for {
		if err := client.Call("Download.GetRate", struct{}{}, &reply); err != nil {
			stderr("get download rate err: %v", err)
			return 1
		}
		fmt.Printf("\rtotal size:%sKB, downloaded:%s", totalSize, reply)
		if reply[0] == "100.00%" {
			break
		}
		time.Sleep(time.Second * time.Duration(5))
	}

	//save aci to rkt store
	s, err := store.NewStore(globalFlags.Dir)
	if err != nil {
		stderr("open rkt store err: %v", err)
		return 1
	}
	aciFile, err := os.Open(aciImage)
	if err != nil {
		stderr("opening ACI file %s err: %v", aciImage, err)
		return 1
	}
	key, err := s.WriteACI(aciFile, true)
	if err != nil {
		stderr("write ACI file err: %v", err)
		return 1
	}
	stdout(key)
	return 0
}

func runFetch(cmd *cobra.Command, args []string) (exit int) {
	// start p2p download if args[0] is a torrent file
	if len(args) > 0 {
		file := strings.TrimSpace(args[0])
		if fileSuffix := path.Ext(file); fileSuffix == ".torrent" {
			if f, err := os.Open(file); err == nil {
				f.Close()
				// use p2p download
				return p2pFetch(args)
			}
		}
	}

	if err := parseApps(&rktApps, args, cmd.Flags(), false); err != nil {
		stderr("fetch: unable to parse arguments: %v", err)
		return 1
	}

	if rktApps.Count() < 1 {
		stderr("fetch: must provide at least one image")
		return 1
	}

	if flagStoreOnly && flagNoStore {
		stderr("both --store-only and --no-store specified")
		return 1
	}

	s, err := store.NewStore(globalFlags.Dir)
	if err != nil {
		stderr("fetch: cannot open store: %v", err)
		return 1
	}
	ks := getKeystore()
	config, err := getConfig()
	if err != nil {
		stderr("fetch: cannot get configuration: %v", err)
		return 1
	}
	ft := &fetcher{
		imageActionData: imageActionData{
			s:             s,
			ks:            ks,
			headers:       config.AuthPerHost,
			dockerAuth:    config.DockerCredentialsPerRegistry,
			insecureFlags: globalFlags.InsecureFlags,
			debug:         globalFlags.Debug,
		},
		storeOnly: flagStoreOnly,
		noStore:   flagNoStore,
		withDeps:  true,
	}

	err = rktApps.Walk(func(app *apps.App) error {
		hash, err := ft.fetchImage(app.Image, app.Asc)
		if err != nil {
			return err
		}
		shortHash := types.ShortHash(hash)
		stdout(shortHash)
		return nil
	})
	if err != nil {
		stderr("%v", err)
		return 1
	}

	return
}
