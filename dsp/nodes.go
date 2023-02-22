package nodes

import (
	"dod/torr/message"
	"encoding/binary"
	"fmt"
	"time"
)

const MaxBlockSize = 16000
const MaxBacklog = 5

func (state *pieceProgress) processMessage() error {
	msg, err := state.client.Read()
	if err != nil {
		return err
	}

	if msg == nil {
		return nil
	}

	switch msg.ID {
	case message.Unchoke:
		state.client.Choked = false
	case message.Choke:
		state.client.Choked = true
	case message.Have:
		index, err := message.ParseHave(msg)
		if err != nil {
			return err
		}
		byteIndex := index / 8
		offset := index % 8
		if !(byteIndex < 0 || byteIndex >= len(state.client.Bitfield)) {
			state.client.Bitfield[byteIndex] |= 1 << uint(7-offset)
		}

	case message.Piece:
		if msg.ID != message.Piece ||
			len(msg.Payload) < 8 {
			return fmt.Errorf("Invalid ID/Payload")
		}
		if int(binary.BigEndian.Uint32(msg.Payload[0:4])) != state.index {
			return fmt.Errorf("Invalid Index")
		}
		if int(binary.BigEndian.Uint32(msg.Payload[4:8])) >= len(state.buf) {
			return fmt.Errorf("Invalid offset")
		}
		if int(binary.BigEndian.Uint32(msg.Payload[4:8]))+len(msg.Payload[8:]) > len(state.buf) {
			return fmt.Errorf("Invalid Data Length")
		}
		copy(state.buf[int(binary.BigEndian.Uint32(msg.Payload[4:8])):], msg.Payload[8:])
		n := len(msg.Payload[8:])
		if err != nil {
			return err
		}
		state.downloaded += n
		state.backlog--
	}
	return nil
}

func DownloadAndCatch(c *message.Client, pw *PieceWork) ([]byte, error) {
	state := pieceProgress{
		index:  pw.Index,
		client: c,
		buf:    make([]byte, pw.Length),
	}
	c.Conn.SetDeadline(time.Now().Add(30 * time.Second))
	defer c.Conn.SetDeadline(time.Time{})
	for state.downloaded < pw.Length {
		if !state.client.Choked {
			for state.backlog < MaxBacklog && state.requested < pw.Length {
				blockSize := MaxBlockSize
				if pw.Length-state.requested < blockSize {
					blockSize = pw.Length - state.requested
				}
				err := c.WriteMsgToConn(pw.Index, state.requested, blockSize)
				if err != nil {
					return nil, err
				}
				state.backlog++
				state.requested += blockSize
			}
		}
		err := state.processMessage()
		if err != nil {
			return nil, err
		}
	}
	return state.buf, nil
}

type PieceWork struct {
	Index  int
	Hash   [20]byte
	Length int
}
type Torrent struct {
	PeerID      [20]byte
	PieceHashes [][20]byte
	Peers       []message.Peer
	PieceLength int
	InfoHash    [20]byte
	Name        string
	Length      int
}

type pieceProgress struct {
	index      int
	client     *message.Client
	buf        []byte
	downloaded int
	requested  int
	backlog    int
}
