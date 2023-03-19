package redis

import (
	"bytes"
	"crypto/sha256"
	"io"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/daulet/replay"

	"go.uber.org/zap"
)

type replayer struct {
	reqLen  int
	lineMux sync.Mutex
	lineBuf *bytes.Buffer
	reqBuf  *bytes.Buffer

	// lock arount 'output' is necessary since
	// both read&write will access it
	outMux    sync.Mutex
	output    *bytes.Buffer
	responses map[[32]byte][][]byte

	log *zap.Logger
}

var _ io.ReadWriteCloser = (*replayer)(nil)

type replayerOption func(*replayer)

func replayerLogger(log *zap.Logger) replayerOption {
	return func(m *replayer) {
		m.log = log
	}
}

func newReplayer(reqFileFunc, respFileFunc replay.FilenameFunc, opts ...replayerOption) (io.ReadWriteCloser, error) {
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

		f, err = os.Open(respFileFunc(reqID))
		if err != nil {
			return nil, err
		}
		resp, err := io.ReadAll(f)
		if err != nil {
			return nil, err
		}
		f.Close()
		hash := sha256.Sum256(req)
		responses[hash] = append(responses[hash], resp)
		reqID++
	}

	m := &replayer{
		lineBuf:   &bytes.Buffer{},
		reqBuf:    &bytes.Buffer{},
		output:    &bytes.Buffer{},
		responses: responses,

		log: zap.NewNop(),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m, nil
}

func (m *replayer) Read(p []byte) (int, error) {
	// we stay on CPU unless we don't imitate latency
	<-time.After(time.Millisecond)
	m.outMux.Lock()
	defer m.outMux.Unlock()
	return m.output.Read(p)
}

func (m *replayer) Write(p []byte) (int, error) {
	m.lineMux.Lock()
	defer m.lineMux.Unlock()
	for _, b := range p {
		m.lineBuf.WriteByte(b)
		if b == '\n' {
			line := m.lineBuf.Bytes()
			m.writeLine(line)
			m.lineBuf.Reset()
		}
	}
	return len(p), nil
}

func (m *replayer) Close() error {
	return nil
}

// Redis request format
// first line contains *<number of parameters>
// what follow is 2 * <number of parameters> lines
// More on data types: https://github.com/go-redis/redis/blob/master/internal/proto/reader.go#L16-L32
func (m *replayer) writeLine(line []byte) (n int, err error) {
	n, err = m.reqBuf.Write(line)
	if m.reqLen == 0 {
		// first line in format "*<int>\t\n"
		m.reqLen, err = strconv.Atoi(string(line[1 : len(line)-2]))
		if err != nil {
			return 0, err
		}
		m.reqLen *= 2 // each parameter is 2 lines
		return
	}
	m.reqLen--
	if m.reqLen > 0 {
		return
	}
	// last line
	req := m.reqBuf.Bytes()
	hash := sha256.Sum256(req)
	m.reqBuf.Reset()
	if resp, ok := m.responses[hash]; ok && len(resp) > 0 {
		oldest := resp[0]
		m.responses[hash] = resp[1:]
		m.outMux.Lock()
		m.output.Write(oldest)
		m.outMux.Unlock()
		return
	}
	m.log.Info("no response found or previously exhaused by the same request", zap.ByteString("hash", hash[:]), zap.String("request", string(req)))
	m.outMux.Lock()
	m.output.Write([]byte("$-1\r\n")) // Null Bulk String
	m.outMux.Unlock()
	return
}
