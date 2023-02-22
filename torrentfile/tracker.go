package torrentfile

import (
	"dod/torr/message"
	"encoding/binary"
	"fmt"
	"net"
	// "net"
	"net/http"
	"net/url"
	// "fmt"
	// "net"
	"strconv"
	"time"

	"github.com/jackpal/bencode-go"
)


func (t *TorStruct) requestPeers(peerID [20]byte, port uint16) ([]message.Peer, error) {
	fmt.Println("requesting for peer list")
	base, err := url.Parse(t.Announce)
	if err != nil {
		return nil, err
	}
	params := url.Values{
		"info_hash":  []string{string(t.InfoHash[:])},
		// "info_hash":  []string{string(t.InfoHash[:])},
		"peer_id":    []string{string(peerID[:])},
		// "port":       []string{strconv.Itoa(int(port))},
		// "uploaded":   []string{"0"},
		"port":       []string{strconv.Itoa(int(port))},
		// "downloaded": []string{"0"},
		"uploaded":   []string{"0"},
		"downloaded": []string{"0"},
		"compact":    []string{"1"},
		"left":       []string{strconv.Itoa(t.Length)},
	}
	base.RawQuery = params.Encode()
	url := base.String()
	if err != nil {
		return nil, err
	}
	fmt.Println("querying:", url)
	c := &http.Client{Timeout: 15 * time.Second}
	resp, err := c.Get(url)
	// if (err != none) {
	// 	return none, err
	// }
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	trackerResp := bencodeTrackerResp{}
	err = bencode.Unmarshal(resp.Body, &trackerResp)
	if err != nil {
		return nil, err
	}
	return Unmarshal([]byte(trackerResp.Peers))
}

func Unmarshal(peersBin []byte) ([]message.Peer, error) {
	const peerSize = 6
	numPeers := len(peersBin) / peerSize
	if len(peersBin)%peerSize != 0 {
		err := fmt.Errorf("INVALID PEERS")
		return nil, err
	}
	peers := make([]message.Peer, numPeers)
	for i := 0; i < numPeers; i++ {
		offset := i * peerSize
		peers[i].IP = net.IP(peersBin[offset : offset+4])
		peers[i].Port = binary.BigEndian.Uint16([]byte(peersBin[offset+4 : offset+6]))
	}
	return peers, nil
}

type bencodeTrackerResp struct {
	Interval int    `bencode:"interval"`
	Peers    string `bencode:"peers"`
}
