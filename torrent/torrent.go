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

func (download *Download) GetDownloadRate(args struct{}, reply *([]string))error {
	rate := fmt.Sprintf("%.2f%c", float64(t.CompletedPieces)*100/float64(len(t.Pieces)),'%')
	*reply = append(*reply, rate)
	return nil
}

func (download *Download) GetDownloadTotalSize(args struct{}, reply *([]string))error {
	size := fmt.Sprintf("%.2f", float64(t.TotalSize/1024))
	*reply = append(*reply, size)
	return nil
}

//only support single file
func (download *Download) GetDownloadFile(args struct{}, reply *([]string))error {
	*reply = append(*reply, t.Files[0].Path)
	return nil
}

func main() {

	var err error
	if len(os.Args) < 2 {
		fmt.Println("Usage: torrent file.torrent")
		return
	}
	
	//start rpc service for client get download rate
	download := new(Download)
	err=rpc.Register(download)
	if err!=nil{
		fmt.Printf("Register download object err,%s\n",err)
		return 
	}	
	rpc.HandleHTTP()
	listener, err := net.Listen("tcp", ":1234")
	if err != nil {
		fmt.Printf("Start rpc service err,%s\n", err)
		return
	}
	defer listener.Close()
	go http.Serve(listener, nil)
	
	// Open torrent file
	if len(os.Args) == 2 {
		t.TimeForExit = 5 //default is 5 minutes exited after finish downlaod
	} else {
		i, err := strconv.Atoi(os.Args[2])
		if err != nil || i <= 0 {
			fmt.Printf("Parameter 2 should be a unsigned integer\n")
			return
		}
		t.TimeForExit = uint32(i)
	}
	if err = t.Open(os.Args[1]); err != nil {
		fmt.Printf("Parse torrent file err,%s\n",err)
		return
	}
	if err=t.Download();err!=nil{
		fmt.Printf("Download err,%s\n",err)
	}

}
