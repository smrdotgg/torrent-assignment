package torrentfile

import (
	"bytes"
	"crypto/rand"
	"crypto/sha1"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"time"

	"github.com/jackpal/bencode-go"

	nodes "dod/torr/dsp"
	"dod/torr/initializer"
	"dod/torr/message"
)

type Torrent struct {
	Peers       []message.Peer
	PeerID      [20]byte
	InfoHash    [20]byte
	PieceHashes [][20]byte
	PieceLength int
	Length      int
	Name        string
}

type TorStruct struct {
	Announce    string
	InfoHash    [20]byte
	PieceHashes [][20]byte
	PieceLength int
	Length      int
	Name        string
}

func New(peer message.Peer, peerID, infoHash [20]byte) (*message.Client, error) {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(peer.IP.String(), strconv.Itoa(int(peer.Port))), 3*time.Second)
	if err != nil {
		return nil, err
	}

	_, err = initializer.CompleteHandshake(conn, infoHash, peerID)
	if err != nil {
		conn.Close()
		return nil, err
	}

	bf, err := recvBitfield(conn)
	if err != nil {
		conn.Close()
		return nil, err
	}

	return &message.Client{
		Conn:     conn,
		Choked:   true,
		Bitfield: bf,
		Peer:     peer,
		InfoHash: infoHash,
		PeerID:   peerID,
	}, nil
}

func recvBitfield(conn net.Conn) (message.Bitfield, error) {
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	defer conn.SetDeadline(time.Time{})

	msg, err := message.Read(conn)
	if err != nil {
		return nil, err
	}
	if msg == nil {
		err := fmt.Errorf("INVALID BITFIELD")
		return nil, err
	}
	if msg.ID != message.BitField {
		err := fmt.Errorf("INVALID BITFIELD")
		return nil, err
	}
	return msg.Payload, nil
}

func (t *Torrent) startDownloadWorker(peer message.Peer, workQueue chan *nodes.PieceWork, results chan *p_result) {

	c, err := New(peer, t.PeerID, t.InfoHash)
	if err != nil {
		log.Printf("[%s] Handshake Error", peer.IP)
		return
	}
	defer c.Conn.Close()
	log.Printf("[%s] Handshake success", peer.IP)

	c.MSG_Unchoke()
	c.MSG_Interested()

	for pw := range workQueue {
		byteIndex := pw.Index / 8
		offset := pw.Index % 8

		if byteIndex < 0 || byteIndex >= len(c.Bitfield) || c.Bitfield[byteIndex]>>uint(7-offset)&1 == 0 {
			workQueue <- pw
			continue
		}

		buf, err := nodes.DownloadAndCatch(c, pw)
		if err != nil {
			log.Println("Exiting", err)
			workQueue <- pw
			return
		}

		err = checkIntegrity(pw, buf)
		if err != nil {
			log.Printf("PIECE BROKEN")
			workQueue <- pw
			continue
		}

		c.MSG_Have(pw.Index)
		results <- &p_result{pw.Index, buf}
	}
}

func checkIntegrity(pw *nodes.PieceWork, buf []byte) error {
	hash := sha1.Sum(buf)
	if !bytes.Equal(hash[:], pw.Hash[:]) {
		return fmt.Errorf("PIECE BROKEN")
	}
	return nil
}

func (t *Torrent) Download() ([]byte, error) {
	log.Println("starting ", t.Name)
	workQueue := make(chan *nodes.PieceWork, len(t.PieceHashes))
	// results := make(chan *nodes.PieceWork, len(t.PieceHashes))
	results := make(chan *p_result)
	for index, hash := range t.PieceHashes {
		begin := index * t.PieceLength
		end := begin + t.PieceLength
		// begin := index * t.PieceLength
		// end := begin + t.PieceLength
		if end > t.Length {
			end = t.Length
		}
		length := end - begin
		workQueue <- &nodes.PieceWork{index, hash, length}
	}
	for _, peer := range t.Peers {
		go t.startDownloadWorker(peer, workQueue, results)
	}
	buf := make([]byte, t.Length)
	donePieces := 0
	for donePieces < len(t.PieceHashes) {
		res := <-results
		begin := res.index * t.PieceLength
		end := begin + t.PieceLength
		if end > t.Length {
			end = t.Length
		}
		copy(buf[begin:end], res.buf)
		donePieces++
		log.Printf("sub piece downloaded")
	}
	close(workQueue)

	return buf, nil
}

func (t *TorStruct) Dl_to_dest(path string) error {
	var peerID [20]byte
	_, err := rand.Read(peerID[:])
	if err != nil {
		return err
	}

	peers, err := t.requestPeers(peerID, 1234)
	if err != nil {
		return err
	}

	torrent := Torrent{
		Peers:  peers,
		PeerID: peerID,
		// InfoHash:    t.InfoHash,
		// PieceHashes: t.PieceHashes,
		// PieceLength: t.PieceLength,
		// Length:      t.Length,
		InfoHash:    t.InfoHash,
		PieceHashes: t.PieceHashes,
		PieceLength: t.PieceLength,
		Length:      t.Length,
		// PieceHashes: t.PieceHashes,
		Name: t.Name,
	}
	buf, err := torrent.Download()
	if err != nil {
		return err
	}

	outFile, err := os.Create(path)
	if err != nil {
		return err
	}
	defer outFile.Close()
	_, err = outFile.Write(buf)
	if err != nil {
		return err
	}
	return nil
}

// bencoded file -> golang struct
func Open(path string) (TorStruct, error) {
	file, err := os.Open(path)
	if err != nil {
		return TorStruct{}, err
	}
	defer file.Close()

	bto := bTor{}
	err = bencode.Unmarshal(file, &bto)
	if err != nil {
		return TorStruct{}, err
	}
	return bto.toTorStruct()
}

func (i *bInf) hash() ([20]byte, error) {
	var buf bytes.Buffer
	err := bencode.Marshal(&buf, *i)
	if err != nil {
		return [20]byte{}, err
	}
	h := sha1.Sum(buf.Bytes())
	return h, nil
}

func (i *bInf) splitPieceHashes() ([][20]byte, error) {
	hashLen := 20
	buf := []byte(i.Pieces)
	if len(buf)%hashLen != 0 {
		err := fmt.Errorf("Received malformed pieces of length %d", len(buf))
		return nil, err
	}
	numHashes := len(buf) / hashLen
	hashes := make([][20]byte, numHashes)

	for i := 0; i < numHashes; i++ {
		copy(hashes[i][:], buf[i*hashLen:(i+1)*hashLen])
	}
	return hashes, nil
}

func (bto *bTor) toTorStruct() (TorStruct, error) {
	infoHash, err := bto.Info.hash()
	if err != nil {
		return TorStruct{}, err
	}
	pieceHashes, err := bto.Info.splitPieceHashes()
	if err != nil {
		return TorStruct{}, err
	}
	t := TorStruct{
		Announce:    bto.Announce,
		InfoHash:    infoHash,
		PieceHashes: pieceHashes,
		PieceLength: bto.Info.PieceLength,
		Length:      bto.Info.Length,
		Name:        bto.Info.Name,
	}
	return t, nil
}

type p_result struct {
	index int
	buf   []byte
}

type p_finish_ratio struct {
	index      int
	client     *message.Client
	buf        []byte
	downloaded int
	requested  int
	backlog    int
}
type bInf struct {
	Pieces      string `bencode:"pieces"`
	PieceLength int    `bencode:"piece length"`
	Length      int    `bencode:"length"`
	Name        string `bencode:"name"`
}

type bTor struct {
	Announce string `bencode:"Announce"`
	Info     bInf   `bencode:"Info"`
}
