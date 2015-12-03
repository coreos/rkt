package torrent

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	TrackerHTTP = iota
	TrackerUDP
)

const (
	TrackerEventNone = iota
	TrackerEventCompleted
	TrackerEventStarted
	TrackerEventStopped
)

type Tracker struct {
	id          int
	announceURL string
	protocol    byte
	interval    int64
	torrent     *Torrent

	announceChannel chan *TrackerRequestData
	stopChannel     chan struct{}
}

type TrackerRequestData struct {
	event      uint32
	downloaded uint64
	uploaded   uint64
	remaining  uint64
}

func (tracker *Tracker) start(data *TrackerRequestData) {
	if err := tracker.announce(data); err != nil {
		//log.Printf("Failed to connect to tracker: %s (%s)\n", tracker.announceURL, err)
		return
	}

	tracker.announceChannel = make(chan *TrackerRequestData)
	tracker.stopChannel = make(chan struct{})
	tracker.torrent.activeTrackerChannel <- tracker.id
	tracker.run()
}

func (tracker *Tracker) run() {
	trackerIntervalTimer := time.Tick(time.Second * time.Duration(tracker.interval))
	for {
		select {
		case data := <-tracker.announceChannel:
			if err := tracker.announce(data); err != nil {
				//log.Print(err)
			}
		case <-tracker.stopChannel:
			tracker.torrent.stoppedTrackerChannel <- tracker.id
			return
		case <-trackerIntervalTimer:
			tracker.torrent.requestAnnounceDataChannel <- tracker.id
		}
	}
}

func (tracker *Tracker) announce(data *TrackerRequestData) (err error) {
	switch tracker.protocol {
	case TrackerHTTP:
		err = tracker.sendHTTPRequest(data)
	case TrackerUDP:
		err = tracker.sendUDPRequest(data)
	}
	return err
}

func (tracker *Tracker) sendUDPRequest(data *TrackerRequestData) error {
	address := tracker.announceURL[6:]
	if slashIndex := strings.IndexByte(address, '/'); slashIndex != -1 {
		address = address[:slashIndex]
	}

	if strings.IndexByte(address, ':') == -1 {
		address += ":6969"
	}

	conn, err := net.Dial("udp", address)
	if err != nil {
		return err
	}

	// Connect
	transactionID := rand.Uint32()

	buf := make([]byte, 16)
	binary.BigEndian.PutUint64(buf, 0x41727101980) // connection id
	binary.BigEndian.PutUint32(buf[8:], 0)         // action
	binary.BigEndian.PutUint32(buf[12:], transactionID)
	if n, err := conn.Write(buf); err != nil {
		return err
	} else if n != 16 {
		return fmt.Errorf("expected to write 16 bytes, wrote %d", n)
	}

	resp := make([]byte, 16)
	if n, err := conn.Read(resp); err != nil {
		return err
	} else if n != 16 {
		return fmt.Errorf("expected packet of length 16, got %d", n)
	}

	if binary.BigEndian.Uint32(resp) != 0 {
		return errors.New("action mismatch")
	} else if binary.BigEndian.Uint32(resp[4:]) != transactionID {
		return errors.New("transaction id mismatch")
	}
	respConnectionID := binary.BigEndian.Uint64(resp[8:])

	// Announce
	buf = make([]byte, 98)
	binary.BigEndian.PutUint64(buf[0:], respConnectionID)
	binary.BigEndian.PutUint32(buf[8:], 1) // action
	binary.BigEndian.PutUint32(buf[12:], transactionID)
	copy(buf[16:], tracker.torrent.getInfoHash())
	copy(buf[36:], tracker.torrent.peerID)

	binary.BigEndian.PutUint64(buf[56:], uint64(data.downloaded))
	binary.BigEndian.PutUint64(buf[64:], uint64(data.remaining))
	binary.BigEndian.PutUint64(buf[72:], uint64(data.uploaded))
	binary.BigEndian.PutUint32(buf[80:], data.event)     // event
	binary.BigEndian.PutUint32(buf[84:], 0)              // ip address
	binary.BigEndian.PutUint32(buf[88:], 0)              // key
	binary.BigEndian.PutUint32(buf[92:], math.MaxUint32) // num_want (-1)
	binary.BigEndian.PutUint16(buf[96:], 6881)           // port
	if n, err := conn.Write(buf); err != nil {
		return fmt.Errorf("failed to write announce message: %s", err)
	} else if n != 98 {
		return fmt.Errorf("expected to write 98 bytes, wrote %d", n)
	}

	resp = make([]byte, 1500)
	n, err := conn.Read(resp)
	if err != nil {
		return fmt.Errorf("failed to read announce response: %s", err)
	} else if n < 20 {
		return fmt.Errorf("expected to read at least 20 bytes, received %d", n)
	}

	if binary.BigEndian.Uint32(resp) != 1 {
		return errors.New("action mismatch")
	} else if binary.BigEndian.Uint32(resp[4:]) != transactionID {
		return errors.New("transaction id mismatch")
	}

	tracker.interval = int64(binary.BigEndian.Uint32(resp[8:]))
	tracker.torrent.peersChannel <- string(resp[20:n])
	return nil
}

func (tracker *Tracker) sendHTTPRequest(data *TrackerRequestData) error {
	var eventString string
	switch data.event {
	case TrackerEventCompleted:
		eventString = "event=completed&"
	case TrackerEventStarted:
		eventString = "event=started&"
	case TrackerEventStopped:
		eventString = "event=stopped&"
	}

	httpResponse, err := http.Get(
		fmt.Sprintf("%s?%speer_id=%s&info_hash=%s&left=%d&compact=1&downloaded=%d&uploaded=%d&port=6881",
			tracker.announceURL,
			eventString,
			url.QueryEscape(string(tracker.torrent.peerID)),
			url.QueryEscape(string(tracker.torrent.getInfoHash())),
			data.remaining,
			data.downloaded,
			data.uploaded,
		),
	)
	if err != nil {
		return err
	}
	defer httpResponse.Body.Close()

	if httpResponse.StatusCode != http.StatusOK {
		return fmt.Errorf("bad response from tracker (%d): %s",
			httpResponse.StatusCode, httpResponse.Status)
	}

	resp, err := Decode(httpResponse.Body)
	if err != nil {
		return err
	}

	if failureReason, exists := resp["failure reason"]; exists {
		return errors.New(failureReason.(string))
	}

	tracker.interval = resp["interval"].(int64)
	tracker.torrent.peersChannel <- resp["peers"]
	return nil
}
