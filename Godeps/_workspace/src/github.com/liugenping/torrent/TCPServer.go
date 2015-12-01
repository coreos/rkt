package torrent

import (
	"fmt"
	"net"
)

type TCPServer struct {
	IP          string
	port        uint16
	connChannel chan net.Conn
	exit        bool
	torrent     *Torrent
}

func NewTCPServer(IP string, port uint16, torrent *Torrent) *TCPServer {

	return &TCPServer{
		IP:          IP,
		port:        port,
		connChannel: make(chan net.Conn),
		torrent:     torrent,
	}
}

func (tcpServer *TCPServer) stop() {
	tcpServer.exit = true

}
func (tcpServer *TCPServer) start() error {
	tcpServer.exit = false
	addr := fmt.Sprintf("%s:%d", tcpServer.IP, tcpServer.port)

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	go tcpServer.handleConnection()
	go func() {
		for !tcpServer.exit {
			conn, err := listener.Accept()
			if err != nil {
				println("Error accept:", err.Error())
			}
			go func() {
				tcpServer.connChannel <- conn
			}()
		}
	}()
	return nil
}

func (tcpServer *TCPServer) handleConnection() {

	for !tcpServer.exit {
		select {
		case conn := <-tcpServer.connChannel:
			peer := &Peer{}

			peer.connection = conn
			peer.torrent = tcpServer.torrent

			peer.remoteChoked = true
			peer.localChoked = true

			peer.localInterested = false
			peer.localInterested = false

			tcpServer.torrent.acceptPeerChannel <- peer

		}
	}
}
