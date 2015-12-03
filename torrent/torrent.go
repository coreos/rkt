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

var t torrent.Torrent

type Download int

func (download *Download) GetDownloadRate(args struct{}, reply *([]string)) error {
	rate := fmt.Sprintf("%.2f%c", float64(t.CompletedPieces)*100/float64(len(t.Pieces)), '%')
	*reply = append(*reply, rate)
	return nil
}

func (download *Download) GetDownloadTotalSize(args struct{}, reply *([]string)) error {
	size := fmt.Sprintf("%.2f", float64(t.TotalSize/1024))
	*reply = append(*reply, size)
	return nil
}

//only support single file
func (download *Download) GetDownloadFile(args struct{}, reply *([]string)) error {
	*reply = append(*reply, t.Files[0].Path)
	return nil
}

func main() {

	var (
		err          error
		listener     net.Listener
		exitDuration int
	)
	if len(os.Args) < 2 {
		fmt.Println("usage: torrent file.torrent duration")
		return
	}

	//start rpc service for client get download rate
	download := new(Download)
	if err = rpc.Register(download); err != nil {
		fmt.Printf("register download object err:%s\n", err)
		return
	}
	rpc.HandleHTTP()
	if listener, err = net.Listen("tcp", ":1234"); err != nil {
		fmt.Printf("start rpc service err:%s\n", err)
		return
	}
	defer listener.Close()
	go http.Serve(listener, nil)

	// Open torrent file
	if len(os.Args) == 2 {
		t.TimeForExit = 10 //default is 5 minutes exited after finish downlaod
	} else {
		if exitDuration, err = strconv.Atoi(os.Args[2]); err != nil || exitDuration < 1 {
			fmt.Printf("parameter 2 should be a unsigned integer\n")
			return
		}
		t.TimeForExit = uint32(exitDuration)
	}
	if err = t.Open(os.Args[1]); err != nil {
		fmt.Printf("parse torrent file err:%s\n", err)
		return
	}
	if err = t.Download(); err != nil {
		fmt.Printf("download err:%s\n", err)
	}

}
