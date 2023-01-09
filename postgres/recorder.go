package postgres

import (
	"encoding/binary"
	"io"
	"net"
	"sync"

	"github.com/daulet/replay"
)

var (
	// Override of "The process ID of this backend.", see message type 'K'.
	fixedProcessID = []byte{0, 0, 0, 33}
	// Override of "The secret key of this backend.", see message type 'K'.
	fixedSecretKey = []byte{2, 4, 8, 16}
)

type recorder struct {
	conn io.ReadWriteCloser

	chIW chan<- byte // Ingress write, before chIR
	chIR <-chan byte // Ingress read, after chIW
	chEW chan<- byte // Egress write, before chER
	chER <-chan byte // Egress read, after chEW
	wg   *sync.WaitGroup

	writer *replay.Writer
	reqTee io.Writer
	resTee io.Writer
}

var _ io.ReadWriteCloser = (*recorder)(nil)

func NewRecorder(addr string, reqFileFunc, respFileFunc replay.FilenameFunc) (io.ReadWriteCloser, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}

	var (
		chIW = make(chan byte)
		chIR = make(chan byte)
		chEW = make(chan byte)
		chER = make(chan byte)
		wg   = &sync.WaitGroup{}
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(chIR)
		startUp(chIW, chIR)
		parseMessages(chIW, chIR)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(chER)
		parseMessages(chEW, chER)
	}()

	writer := replay.NewWriter(reqFileFunc, respFileFunc)
	reqTee := writer.RequestWriter()
	resTee := writer.ResponseWriter()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for b := range chIR {
			bs := []byte{b}
			reqTee.Write(bs)
			conn.Write(bs)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for b := range chER {
			resTee.Write([]byte{b})
		}
	}()

	return &recorder{
		conn: conn,

		chIW: chIW,
		chIR: chIR,
		chEW: chEW,
		chER: chER,
		wg:   wg,

		writer: writer,
		reqTee: reqTee,
		resTee: resTee,
	}, nil
}

// Start-up phase as described in https://www.postgresql.org/docs/current/protocol-overview.html
func startUp(chW chan byte, chR chan<- byte) {
	buf := readN(chW, 4)
	length := int(binary.BigEndian.Uint32(buf))
	for i := 0; i < length-4; i++ {
		buf = append(buf, <-chW)
	}
	writeN(chR, deterministicStartup(buf))
}

// Normal phase as described in https://www.postgresql.org/docs/current/protocol-overview.html
func parseMessages(chW chan byte, chR chan<- byte) {
	// Parse messages: https://www.postgresql.org/docs/current/protocol-message-formats.html
	for b := range chW {
		writeN(chR, []byte{b}) // message type
		lenBytes := readN(chW, 4)
		length := int(binary.BigEndian.Uint32(lenBytes))
		writeN(chR, lenBytes) // length

		switch b {
		case 'X': // Terminate
			return
		case 'K':
			for range fixedProcessID {
				<-chW
			}
			for range fixedSecretKey {
				<-chW
			}
			writeN(chR, fixedProcessID)
			writeN(chR, fixedSecretKey)
		default:
			// length value includes itself and NULL terminator
			writeN(chR, readN(chW, length-4))
		}
	}
}

func (r *recorder) Read(p []byte) (int, error) {
	n, err := r.conn.Read(p)
	if err != nil {
		return n, err
	}
	p = p[:n]
	for _, b := range p {
		r.chEW <- b
	}
	return n, err
}

func (r *recorder) Write(p []byte) (int, error) {
	for _, b := range p {
		r.chIW <- b
	}
	// TODO how do we handle EOF?
	return len(p), nil
}

func (r *recorder) Close() error {
	r.conn.Close()
	// TODO these should be closed due to EOF on conns, not out of the loop;
	close(r.chIW)
	close(r.chEW)
	r.wg.Wait()
	r.writer.Close()
	return nil
}
