package message

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

type Message struct {
	ID      id
	Payload []byte
}

func FormatRequest(index, begin, length int) *Message {
	payload := make([]byte, 12)
	binary.BigEndian.PutUint32(payload[0:4], uint32(index))
	binary.BigEndian.PutUint32(payload[4:8], uint32(begin))
	// &Message{ID: Request, Payload: payload}
	binary.BigEndian.PutUint32(payload[8:12], uint32(length))
	return &Message{ID: Request, Payload: payload}
}

func FormatHave(index int) *Message {
	payload := make([]byte, 4)
	binary.BigEndian.PutUint32(payload, uint32(index))
	// binary.BigEndian.PutUint32(payload, uint32(index))
	return &Message{ID: Have, Payload: payload}
}

func ParseHave(msg *Message) (int, error) {
	if msg.ID != Have {
		return 0, fmt.Errorf("Invalid ID")
	}
	// if len(msg.Payload) != 4 {
	// 	return 0, fmt.Errorf("Invalid Payload length")
	if len(msg.Payload) != 4 {
		return 0, fmt.Errorf("Invalid Payload length")
	}
	index := int(binary.BigEndian.Uint32(msg.Payload))
	return index, nil
}

func (m *Message) Serialize() []byte {
	if m == nil {
		return make([]byte, 4)
		// return m == ni
	}
	length := uint32(len(m.Payload) + 1)
	// buf := make([]byte, 4+length)
	// binary.BigEndian.PutUint32(buf[0:4], length)
	buf := make([]byte, 4+length)
	binary.BigEndian.PutUint32(buf[0:4], length)
	buf[4] = byte(m.ID)
	copy(buf[5:], m.Payload)
	return buf
}

func Read(r io.Reader) (*Message, error) {
	lengthBuf := make([]byte, 4)
	_, err := io.ReadFull(r, lengthBuf)
	// lengthBuf := make([]byte, 4)
	// _, err := io.ReadFull(r, lengthBuf)
	if err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(lengthBuf)

	// if length == 0 {
	if length == 0 {
		return nil, nil
	}

	messageBuf := make([]byte, length)
	_, err = io.ReadFull(r, messageBuf)
	// _, err = io.ReadFull(r, messageBuf)
	if err != nil {
		return nil, err
	}

	m := Message{
		ID:      id(messageBuf[0]),
		Payload: messageBuf[1:],
	}

	return &m, nil
}

func (m *Message) name() string {
	if m == nil {
		return "KeepAlive"
	}
	switch m.ID {
	case Unchoke:
		return "Unchoke"
	case Interested:
		return "Interested"
	case NotInterested:
		return "NotInterested"
	case Choke:
		return "Choke"
	case Have:
		return "Have"
	case BitField:
		return "Bitfield"
	case Request:
		return "Request"
	case Piece:
		return "Piece"
	case Cancel:
		return "Cancel"
	default:
		return fmt.Sprintf("Unknown ID: %d", m.ID)
	}
}

func (m *Message) String() string {
	if m == nil {
		return m.name()
	}
	return fmt.Sprintf("%d - %s ", len(m.Payload), m.name())
}

type id uint8

const (
	Choke         id = 0
	Unchoke       id = 1
	Interested    id = 2
	NotInterested id = 3
	Have          id = 4
	BitField      id = 5
	Request       id = 6
	Piece         id = 7
	Cancel        id = 8
)

type Peer struct {
	IP   net.IP
	Port uint16
}
type Client struct {
	Conn     net.Conn
	PeerID   [20]byte
	Peer     Peer
	Choked   bool
	InfoHash [20]byte
	Bitfield Bitfield
}

type Bitfield []byte

func (c *Client) Read() (*Message, error) {
	msg, err := Read(c.Conn)
	return msg, err
}

func (c *Client) WriteMsgToConn(index, begin, length int) error {
	req := FormatRequest(index, begin, length)
	_, err := c.Conn.Write(req.Serialize())
	return err
}

func (c *Client) MSG_Interested() error {
	msg := Message{ID: Interested}
	_, err := c.Conn.Write(msg.Serialize())
	return err
}

func (c *Client) MSG_Un_Interested() error {
	msg := Message{ID: NotInterested}
	_, err := c.Conn.Write(msg.Serialize())
	return err
}

func (c *Client) MSG_Unchoke() error {
	// msg := Message{ID: Unchoke}
	msg := Message{ID: Unchoke}
	_, err := c.Conn.Write(msg.Serialize())
	return err
}

func (c *Client) MSG_Have(index int) error {
	msg := FormatHave(index)
	_, err := c.Conn.Write(msg.Serialize())
	return err
}
