package postgres

import (
	"crypto/sha256"
	"encoding/binary"
	"io"
	"os"
	"sync"

	"github.com/daulet/replay"

	"go.uber.org/zap"
)

type replayer struct {
	wg      *sync.WaitGroup
	readCh  chan byte
	writeCh chan byte
}

var _ io.ReadWriteCloser = (*replayer)(nil)

func newReplayer(log *zap.Logger, reqFileFunc, respFileFunc replay.FilenameFunc) (io.ReadWriteCloser, error) {
	var (
		reqID     int
		err       error
		responses = make(map[[32]byte][][]byte)
	)
	for err == nil {
		f, err := os.Open(reqFileFunc(reqID))
		if err != nil {
			break
		}
		req, err := io.ReadAll(f)
		if err != nil {
			return nil, err
		}
		f.Close()

		// TODO is no response okay? Maybe we are closing inside tests too soon?
		var resp []byte
		{
			f, err = os.Open(respFileFunc(reqID))
			// no response is okay
			if err == nil {
				resp, err = io.ReadAll(f)
				if err != nil {
					return nil, err
				}
				f.Close()
			}
		}
		hash := sha256.Sum256(req)
		responses[hash] = append(responses[hash], resp)
		reqID++
	}

	var wg sync.WaitGroup
	r := &replayer{
		wg:      &wg,
		readCh:  make(chan byte),
		writeCh: make(chan byte),
	}
	wg.Add(1)
	go func() {
		defer r.wg.Done()
		defer close(r.writeCh)
		defer close(r.readCh)
		process(log, r.readCh, r.writeCh, responses)
	}()
	return r, nil
}

func process(
	log *zap.Logger,
	readCh chan<- byte,
	writeCh <-chan byte,
	responses map[[32]byte][][]byte,
) {
	var (
		firstReq = true
	)
	for {
		var (
			buf     []byte
			msgType byte
		)
		if !firstReq {
			msgType = <-writeCh
			buf = append(buf, msgType)
		}

		buf = append(buf, readN(writeCh, 4)...)
		length := int(binary.BigEndian.Uint32(buf[len(buf)-4:]))
		buf = append(buf, readN(writeCh, length-4)...)

		if firstReq {
			buf = deterministicStartup(buf)
		}
		firstReq = false

		req := string(buf)
		hash := sha256.Sum256(buf)

		if resp, ok := responses[hash]; ok && len(resp) > 0 {
			oldest := resp[0]
			responses[hash] = resp[1:]
			writeN(readCh, oldest)
		} else {
			// TODO write something in error case? is there error response?
			log.Info("no response found or previously exhaused by the same request", zap.ByteString("hash", hash[:]), zap.String("request", req))
		}

		if msgType == 'X' {
			break
		}
	}
}

func (r *replayer) Read(p []byte) (int, error) {
	n := 0
	for {
		select {
		case b, ok := <-r.readCh:
			if !ok {
				return n, io.EOF
			}
			p[n] = b
			n++
			if n == len(p) {
				return n, nil
			}
		default:
			return n, nil
		}
	}
}

func (r *replayer) Write(p []byte) (int, error) {
	for _, b := range p {
		r.writeCh <- b
	}
	return len(p), nil
}

func (r *replayer) Close() error {
	r.wg.Wait()
	return nil
}
