package redisreplay

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"
)

type matcher struct {
	reqLen    int
	buffer    *bytes.Buffer
	output    *bytes.Buffer
	responses map[[32]byte][]byte
}

// TODO unexport functions that don't need to be exported
// TODO better, controlled logging
func NewMatcher() (*matcher, error) {
	// TODO feed from user
	dir := "testdata"
	var (
		reqID     int
		err       error
		responses = make(map[[32]byte][]byte)
	)
	for err == nil {
		f, err := os.Open(fmt.Sprintf("%s/%d.request", dir, reqID))
		if err != nil {
			break
		}
		req, err := io.ReadAll(f)
		if err != nil {
			return nil, err
		}
		f.Close()

		f, err = os.Open(fmt.Sprintf("%s/%d.response", dir, reqID))
		if err != nil {
			return nil, err
		}
		resp, err := io.ReadAll(f)
		if err != nil {
			return nil, err
		}
		f.Close()
		hash := sha256.Sum256(req)
		// TODO could be multiple responses for the same request, preserve order
		responses[hash] = resp
		fmt.Printf("loaded request %d, hash %x\n", reqID, hash)
		fmt.Println(string(req))
		reqID++
	}

	return &matcher{
		buffer:    &bytes.Buffer{},
		output:    &bytes.Buffer{},
		responses: responses,
	}, nil
}

func (m *matcher) Read(p []byte) (int, error) {
	// we stay on CPU unless we don't imitate latency
	<-time.After(time.Millisecond)
	// Read can return EOF
	n, _ := m.output.Read(p)
	return n, nil
}

// Redis request format
// first line contains *<number of parameters>
// what follow is 2 * <number of parameters> lines
func (m *matcher) WriteLine(line []byte) (n int, err error) {
	n, err = m.buffer.Write(line)
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
	req := m.buffer.Bytes()
	hash := sha256.Sum256(req)
	m.buffer.Reset()
	if resp, ok := m.responses[hash]; ok {
		m.output.Write(resp)
		return
	}
	fmt.Printf("no response found for request %x\n", hash)
	fmt.Println(string(req))
	// Null Bulk String
	m.output.Write([]byte("$-1\r\n"))
	return
}
