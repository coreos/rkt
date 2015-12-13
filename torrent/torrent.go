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
	"net/http"
	"net/rpc"
	"os"
	"strconv"

	"github.com/coreos/rkt/Godeps/_workspace/src/github.com/liugenping/torrent"
)

type Download struct {
	t torrent.Torrent
}

func (d *Download) GetRate(args struct{}, reply *([]string)) error {
	rate := fmt.Sprintf("%.2f%c", float64(d.t.CompletedPieces)*100/float64(len(d.t.Pieces)), '%')
	*reply = append(*reply, rate)
	return nil
}

func (d *Download) GetTotalSize(args struct{}, reply *([]string)) error {
	size := fmt.Sprintf("%.2f", float64(d.t.TotalSize/1024))
	*reply = append(*reply, size)
	return nil
}

func (d *Download) GetFile(args struct{}, reply *([]string)) error {
	// only support single file
	*reply = append(*reply, d.t.Files[0].Path)
	return nil
}

func main() {
	if len(os.Args) < 2 {
		// cmmand should like "torrent  etcd.torrent 10"
		fmt.Println("usage: torrent file.torrent duration")
		return
	}

	// start rpc service for client get download rate
	download := &Download{}
	if err := rpc.Register(download); err != nil {
		fmt.Printf("register download object err:%s\n", err)
		return
	}
	rpc.HandleHTTP()
	listener, err := net.Listen("tcp", ":1234")
	if err != nil {
		fmt.Printf("start rpc service err:%s\n", err)
		return
	}
	defer listener.Close()
	go http.Serve(listener, nil)

	// set duration for torrent exit
	if len(os.Args) == 2 {
		download.t.TimeForExit = 5 //default is 5 minutes exited after finish downlaod
	} else {
		exitDuration, err := strconv.Atoi(os.Args[2])
		if err != nil || exitDuration < 1 {
			fmt.Printf("parameter 2 should be a unsigned integer\n")
			return
		}
		download.t.TimeForExit = uint32(exitDuration)
	}

	// parse torrent file
	if err := download.t.Open(os.Args[1]); err != nil {
		fmt.Printf("parse torrent file err:%s\n", err)
		return
	}

	// downlaod
	if err := download.t.Download(); err != nil {
		fmt.Printf("download err:%s\n", err)
	}

}
