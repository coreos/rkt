// Copyright 2015 The rkt Authors
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
	"net"
	"os/signal"
	"syscall"

	"github.com/coreos/go-systemd/activation"
	"github.com/coreos/rkt/api/v1alpha"
	"github.com/coreos/rkt/common"
	"github.com/coreos/rkt/store/imagestore"
	"github.com/hashicorp/errwrap"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

const (
	// Default size for the pod LRU cache.
	defaultPodCacheSize = 512
	// Default size for the image LRU cache.
	defaultImageCacheSize = 512
)

var (
	supportedAPIVersion = "1.0.0-alpha"
	cmdAPIService       = &cobra.Command{
		Use:   fmt.Sprintf(`api-service [--listen="%s"]`, common.APIServiceListenAddr),
		Short: "Run API service (experimental)",
		Long: fmt.Sprintf(`The API service listens for gRPC requests on the address and port specified by
the --listen option, by default %s

Specify the address 0.0.0.0 to listen on all interfaces.

It will also run correctly as a systemd socket-activated service, see
systemd.socket(5).`, common.APIServiceListenAddr),
		Run: runWrapper(runAPIService),
	}

	flagAPIServiceListenAddr string
	flagSocketListen         bool
	systemdFDs               = activation.Files // for mocking
)

func init() {
	cmdRkt.AddCommand(cmdAPIService)
	cmdAPIService.Flags().StringVar(&flagAPIServiceListenAddr, "listen", "", "address to listen for client API requests")
}

// Open one or more listening sockets, then start the gRPC server
func runAPIService(cmd *cobra.Command, args []string) (exit int) {
	// Set up the signal handler here so we can make sure the
	// signals are caught after print the starting message.
	signal.Notify(exitCh, syscall.SIGINT, syscall.SIGTERM)

	stderr.Print("API service starting...")

	listeners, err := openAPISockets()
	if err != nil {
		stderr.PrintE("Failed to open sockets", err)
		return 1
	}
	if len(listeners) == 0 { // This is unlikely...
		stderr.Println("No sockets to listen to. Quitting.")
		return 1
	}

	publicServer := grpc.NewServer() // TODO(yifan): Add TLS credential option.

	store, err := imagestore.NewStore(storeDir())
	if err != nil {
		stderr.Println("Cannot open store", err)
		return 1
	}

	v1alphaReadOnlyAPIServer, err := newV1alphaReadOnlyAPIServer(store)
	if err != nil {
		stderr.PrintE("failed to create read-only API service", err)
		return 1
	}

	v1alpha.RegisterPublicAPIServer(publicServer, v1alphaReadOnlyAPIServer)

	// Register write-only APIs only if all the sockets are unix sockets.
	allUnixListeners := true
loop:
	for _, l := range listeners {
		switch l.(type) {
		case *net.UnixListener:
		default:
			allUnixListeners = false
			break loop
		}
	}
	if allUnixListeners {
		v1alphaWriteOnlyAPIServer, err := newV1alphaWriteOnlyAPIServer(store)
		if err != nil {
			stderr.PrintE("failed to create write-only API service", err)
			return 1
		}
		v1alpha.RegisterPublicWriteOnlyAPIServer(publicServer, v1alphaWriteOnlyAPIServer)
	}

	for _, l := range listeners {
		defer l.Close()
		go publicServer.Serve(l)
	}

	stderr.Printf("API service running")

	<-exitCh

	stderr.Print("API service exiting...")

	return
}

// Open API sockets based on command line parameters and
// the magic environment variable from systemd
//
// see sd_listen_fds(3)
func openAPISockets() ([]net.Listener, error) {
	listeners := []net.Listener{}

	fds := systemdFDs(true) // Try to get the socket fds from systemd
	if len(fds) > 0 {
		if flagAPIServiceListenAddr != "" {
			return nil, fmt.Errorf("started under systemd.socket(5), but --listen passed! Quitting.")
		}

		stderr.Printf("Listening on %d systemd-provided socket(s)\n", len(fds))
		for _, fd := range fds {
			l, err := net.FileListener(fd)
			if err != nil {
				return nil, errwrap.Wrap(fmt.Errorf("could not open listener"), err)
			}
			listeners = append(listeners, l)
		}
	} else {
		if flagAPIServiceListenAddr == "" {
			flagAPIServiceListenAddr = common.APIServiceListenAddr
		}
		stderr.Printf("Listening on %s\n", flagAPIServiceListenAddr)

		var errtcp, errunix error
		var l net.Listener
		l, errtcp = net.Listen("tcp", flagAPIServiceListenAddr)
		if errtcp != nil {
			l, errunix = net.Listen("unix", flagAPIServiceListenAddr)
			if errunix != nil {
				return nil, fmt.Errorf("could not create TCP listener: %v, or Unix listener: %v", errtcp, errunix)
			}
		}
		listeners = append(listeners, l)
	}

	return listeners, nil
}
