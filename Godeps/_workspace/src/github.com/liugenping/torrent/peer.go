package torrent

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const (
	Choke = iota
	Unchoke
	Interested
	Uninterested
	Have
	Bitfield
	Request
	PieceBlock
	Cancel
	Port
)

type Peer struct {
	ip      net.IP
	port    uint16
	torrent *Torrent

	pieces           map[uint32]struct{}
	queue            []*PeerPiece
	connection       net.Conn
	remoteChoked     bool
	remoteInterested bool
	localChoked      bool
	localInterested  bool
	closed           uint32

	requestPieceChannel   chan uint32
	sendPieceBlockChannel chan *BlockMessage
	sendHaveChannel       chan uint32
	done                  chan struct{}

	queueLock sync.Mutex

	id string
}

type PeerPiece struct {
	index     uint32
	blocks    [][]byte
	writes    int
	reqWrites int
}

type BlockMessage struct {
	index uint32
	begin uint32
	block []byte
}

func NewPeer(torrent *Torrent) *Peer {
	peer := Peer{}
	peer.torrent = torrent

	peer.remoteChoked = true
	peer.localChoked = true
	return &peer
}

func (peer *Peer) readN(n int) ([]byte, error) {
	buf := make([]byte, n)
	_, err := io.ReadFull(peer.connection, buf)
	return buf, err
}

func (peer *Peer) close() {
	if atomic.CompareAndSwapUint32(&peer.closed, 0, 1) {
		peer.connection.Close()
	}
}

func (peer *Peer) connect() {
	defer func() {
		peer.torrent.decrementPeerCount <- struct{}{}
	}()

	var addr string
	if ip := peer.ip.To4(); ip != nil {
		addr = fmt.Sprintf("%s:%d", ip.String(), peer.port)
	} else {
		addr = fmt.Sprintf("[%s]:%d", peer.ip.String(), peer.port)
	}

	var err error
	if peer.connection, err = net.DialTimeout("tcp", addr, time.Second*5); err != nil {
		return
	}
	defer peer.close()

	// Send handshake
	if _, err := peer.connection.Write(peer.torrent.handshake); err != nil {
		//log.Printf("failed to send handshake to peer: %s\n", err)
		return
	}

	// Receive handshake
	if handshake, err := peer.readN(68); err != nil {
		//log.Printf("failed to read handshake from peer: %s\n", err)
		return
	} else if !bytes.Equal(handshake[0:20], []byte("\x13BitTorrent protocol")) {
		//log.Printf("bad protocol from peer: %s\n", addr)
		return
	} else if !bytes.Equal(handshake[28:48], peer.torrent.getInfoHash()) {
		//log.Printf("info hash mismatch from peer: %s\n", addr)
		return
	} else if len(peer.id) != 0 {
		if !bytes.Equal(handshake[48:68], []byte(peer.id)) {
			//log.Printf("peer id mismatch from peer: %s\n", addr)
			return
		}
	} else {
		peer.id = string(handshake[48:68])
	}

	peer.requestPieceChannel = make(chan uint32)
	peer.sendPieceBlockChannel = make(chan *BlockMessage)
	peer.sendHaveChannel = make(chan uint32)
	peer.done = make(chan struct{})
	peer.pieces = make(map[uint32]struct{})

	peer.torrent.addPeerChannel <- peer

	errorChannel := make(chan struct{})
	go peer.receiver(errorChannel)

	for {
		select {
		case pieceIndex := <-peer.requestPieceChannel:
			peer.sendPieceRequest(pieceIndex)
		case blockMessage := <-peer.sendPieceBlockChannel:
			peer.sendPieceBlockMessage(blockMessage)
		case pieceIndex := <-peer.sendHaveChannel:
			peer.sendHaveMessage(pieceIndex)
		case <-errorChannel:
			go func() {
				peer.torrent.removePeerChannel <- peer
			}()
			for {
				select {
				// Until we receive another message over "done" to
				// confirm the removal of the peer, it's possible that
				// Torrent can send messages over other channels.
				case <-peer.sendHaveChannel:
				case <-peer.sendPieceBlockChannel:
				case <-peer.requestPieceChannel:
				case <-peer.done:
					return
				}
			}
		case <-peer.done:
			peer.close()
		}
	}
}

func (peer *Peer) receiver(errorChannel chan struct{}) {
	for {
		lengthHeader, err := peer.readN(4)
		if err != nil {
			errorChannel <- struct{}{}
			break
		}

		length := binary.BigEndian.Uint32(lengthHeader)
		if length == 0 {
			// keep-alive
			continue
		}

		data, err := peer.readN(int(length))
		if err != nil {
			errorChannel <- struct{}{}
			break
		}

		if err := peer.processMessage(length, data[0], data[1:]); err != nil {
			//log.Print(err)
			errorChannel <- struct{}{}
			break
		}
	}
}

func (peer *Peer) processMessage(length uint32, messageType byte, payload []byte) error {
	switch messageType {
	case Choke:
		if length != 1 {
			return errors.New("length of choke packet must be 1")
		}

		peer.remoteChoked = true
	case Unchoke:
		if length != 1 {
			return errors.New("length of unchoke packet must be 1")
		}

		peer.remoteChoked = false
		peer.queueLock.Lock()
		for _, piece := range peer.queue {
			peer.requestPiece(piece)
		}
		peer.queueLock.Unlock()
	case Interested:
		if length != 1 {
			return errors.New("length of interested packet must be 1")
		}

		peer.remoteInterested = true
		peer.sendUnchokeMessage()
	case Uninterested:
		if length != 1 {
			return errors.New("length of not interested packet must be 1")
		}

		peer.remoteInterested = false
	case Have:
		if length != 5 {
			return errors.New("length of have packet must be 5")
		}

		index := binary.BigEndian.Uint32(payload)
		peer.torrent.havePieceChannel <- &HavePieceMessage{peer, index}
	case Bitfield:
		if length < 2 {
			return errors.New("length of bitfield packet must be at least 2")
		}
		peer.torrent.bitfieldChannel <- &BitfieldMessage{peer, payload}
	case Request:
		if length != 13 {
			return errors.New("length of request packet must be 13")
		}

		if !peer.remoteInterested {
			return errors.New("peer sent request without showing interest")
		}

		if peer.localChoked {
			return errors.New("peer sent request while choked")
		}

		index := binary.BigEndian.Uint32(payload)
		begin := binary.BigEndian.Uint32(payload[4:])
		blockLength := binary.BigEndian.Uint32(payload[8:])
		if blockLength > 32768 {
			return errors.New("peer requested length over 32KB")
		}

		peer.torrent.blockRequestChannel <- &BlockRequestMessage{peer, index, begin, blockLength}
	case PieceBlock:
		if length < 10 {
			return errors.New("length of piece packet must be at least 10")
		}

		blockLength := length - 9
		if blockLength > 16384 {
			return errors.New("received block over 16KB")
		}

		index := binary.BigEndian.Uint32(payload)

		peer.queueLock.Lock()
		defer peer.queueLock.Unlock()

		idx := 0
		var piece *PeerPiece
		for k, v := range peer.queue {
			if v.index == index {
				idx = k
				piece = v
				break
			}
		}

		if piece == nil {
			return errors.New("received index we didn't ask for")
		}

		begin := binary.BigEndian.Uint32(payload[4:])

		blockIndex := begin / 16384
		if int(blockIndex) >= len(piece.blocks) {
			return errors.New("received too big block index")
		}
		piece.blocks[blockIndex] = payload[8:]

		piece.writes++
		if piece.writes == piece.reqWrites {
			// Glue all blocks together into a piece
			pieceData := []byte{}
			for k := range piece.blocks {
				pieceData = append(pieceData, piece.blocks[k]...)
			}

			// Send piece to Torrent
			peer.torrent.pieceChannel <- &PieceMessage{peer, index, pieceData}

			// Remove piece from peer
			peer.queue = append(peer.queue[:idx], peer.queue[idx+1:]...)
		}
	case Cancel:
		if length != 13 {
			return errors.New("length of cancel packet must be 13")
		}

		// TODO: Handle cancel
		/*
		   index := binary.BigEndian.Uint32(payload)
		   begin := binary.BigEndian.Uint32(payload[4:])
		   length := binary.BigEndian.Uint32(payload[8:])
		*/
	case Port:
		if length != 3 {
			return errors.New("length of port packet must be 3")
		}

		// port := binary.BigEndian.Uint16(payload)
		// Peer has a DHT node on port
	}
	return nil
}

func (peer *Peer) sendInterested() {
	if peer.localInterested {
		return
	}

	peer.connection.Write([]byte{0, 0, 0, 1, Interested})
	peer.localInterested = true
}

func (peer *Peer) sendRequest(index, begin, length uint32) {
	packet := make([]byte, 17)
	binary.BigEndian.PutUint32(packet, 13) // Length
	packet[4] = Request
	binary.BigEndian.PutUint32(packet[5:], index)
	binary.BigEndian.PutUint32(packet[9:], begin)
	binary.BigEndian.PutUint32(packet[13:], length)
	peer.connection.Write(packet)
}

func (peer *Peer) sendPieceRequest(index uint32) {
	peer.sendInterested()

	pieceLength := peer.torrent.getPieceLength(index)
	blocks := int(math.Ceil(float64(pieceLength) / 16384))
	piece := &PeerPiece{index, make([][]byte, blocks), 0, blocks}
	peer.queueLock.Lock()
	peer.queue = append(peer.queue, piece)
	peer.queueLock.Unlock()
	if !peer.remoteChoked {
		peer.requestPiece(piece)
	}
}

func (peer *Peer) requestPiece(piece *PeerPiece) {
	var pos uint32
	pieceLength := peer.torrent.getPieceLength(piece.index)
	for pieceLength > 16384 {
		peer.sendRequest(piece.index, pos, 16384)
		pieceLength -= 16384
		pos += 16384
	}
	peer.sendRequest(piece.index, pos, uint32(pieceLength))
}

func (peer *Peer) sendPieceBlockMessage(blockMessage *BlockMessage) {
	packet := make([]byte, 13)
	binary.BigEndian.PutUint32(packet, uint32(9+len(blockMessage.block)))
	packet[4] = PieceBlock
	binary.BigEndian.PutUint32(packet[5:], blockMessage.index)
	binary.BigEndian.PutUint32(packet[9:], blockMessage.begin)
	peer.connection.Write(packet)
	peer.connection.Write(blockMessage.block)
}

func (peer *Peer) sendHaveMessage(pieceIndex uint32) {
	packet := make([]byte, 9)
	packet[3] = 5 // Length
	packet[4] = Have
	binary.BigEndian.PutUint32(packet[5:], pieceIndex)
	peer.connection.Write(packet)
}

func (peer *Peer) sendUnchokeMessage() {
	if !peer.localChoked {
		return
	}

	peer.connection.Write([]byte{0, 0, 0, 1, Unchoke})
	peer.localChoked = false
}

func (peer *Peer) handleHandshake() error {
	var addr string
	if handshake, err := peer.readN(68); err != nil {
		return fmt.Errorf("failed to read handshake from peer: %s\n", err)
	} else if !bytes.Equal(handshake[0:20], []byte("\x13BitTorrent protocol")) {
		return fmt.Errorf("bad protocol from peer: %s\n", addr)
	} else if !bytes.Equal(handshake[28:48], peer.torrent.getInfoHash()) {
		return fmt.Errorf("info hash mismatch from peer: %s\n", addr)
	} else if len(peer.id) != 0 {
		if !bytes.Equal(handshake[48:68], []byte(peer.id)) {
			return fmt.Errorf("peer id mismatch from peer: %s\n", addr)
		}
	} else {
		peer.id = string(handshake[48:68])
	}
	return nil
}

//handShake: <pstrlen><pstr><reserved><info_hash><peer_id>
func (peer *Peer) sendHandshake() error {
	// Set handshake
	var buffer bytes.Buffer
	buffer.WriteByte(19) // length of the string "BitTorrent Protocol"
	buffer.WriteString("BitTorrent protocol")
	buffer.WriteString("\x00\x00\x00\x00\x00\x00\x00\x00") // reserved
	buffer.Write(peer.torrent.getInfoHash())
	buffer.Write(peer.torrent.peerID[:])

	// Send handshake
	if _, err := peer.connection.Write(buffer.Bytes()); err != nil {
		return fmt.Errorf("failed to send handshake to peer: %s\n", err)
	}
	return nil
}

func (peer *Peer) handleAcception() {

	errorChannel := make(chan struct{})
	go peer.receiver(errorChannel)

	for {
		select {
		case pieceIndex := <-peer.requestPieceChannel:
			peer.sendPieceRequest(pieceIndex)
		case blockMessage := <-peer.sendPieceBlockChannel:
			peer.sendPieceBlockMessage(blockMessage)
		case pieceIndex := <-peer.sendHaveChannel:
			peer.sendHaveMessage(pieceIndex)
		case <-errorChannel:
			go func() {
				peer.torrent.removePeerChannel <- peer
			}()
			for {
				select {
				// Until we receive another message over "done" to
				// confirm the removal of the peer, it's possible that
				// Torrent can send messages over other channels.
				case <-peer.sendHaveChannel:
				case <-peer.sendPieceBlockChannel:
				case <-peer.requestPieceChannel:
				case <-peer.done:
					return
				}
			}
		case <-peer.done:
			peer.close()
		}
	}
}
