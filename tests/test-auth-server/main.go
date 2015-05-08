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
	"os"

	taas "github.com/coreos/rkt/tests/test-auth-server/aci"
)

func main() {
	cmdsStr := "start, stop"
	if len(os.Args) < 2 {
		fmt.Printf("Error: expected a command - %s\n", cmdsStr)
		os.Exit(1)
	}
	var err error
	switch os.Args[1] {
	case "start":
		err = start(os.Args[2:])
	case "stop":
		err = stop(os.Args[2:])
	default:
		err = fmt.Errorf("wrong command %q, should be %s", os.Args[1], cmdsStr)
	}
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func start(args []string) error {
	typesStr := "none, basic, oauth"
	if len(args) < 1 {
		return fmt.Errorf("expected a type - %s", typesStr)
	}
	types := map[string]taas.Type{
		"none":  taas.None,
		"basic": taas.Basic,
		"oauth": taas.Oauth,
	}
	auth, ok := types[args[0]]
	if !ok {
		return fmt.Errorf("wrong type %q, should, be %s", args[0], typesStr)
	}
	server, err := taas.StartServer(auth)
	if err != nil {
		return fmt.Errorf("failed to start server: %v", err)
	}
	if server.Conf != "" {
		fmt.Printf(server.Conf)
	}
	fmt.Printf("Ready, waiting for connections at %s\n", server.URL)
	loop(server)
	fmt.Println("Byebye")
	return nil
}

func loop(server *taas.Server) {
	for {
		select {
		case <-server.Stop:
			server.Close()
			return
		case msg, ok := <-server.Msg:
			if ok {
				fmt.Println(msg)
			}
		}
	}
}

func stop(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("expected a host")
	}
	host := args[0]
	res, err := taas.StopServer(host)
	if err != nil {
		return fmt.Errorf("failed to stop server: %v", err)
	}
	defer res.Body.Close()
	fmt.Printf("Response status: %s\n", res.Status)
	if res.StatusCode/100 != 2 {
		return fmt.Errorf("got a nonsuccess status")
	}
	return nil
}
