package redisreplay

import (
	"io"
	"net"
	"os"
)

// TODO need to differentiate between request and response callers
type Recorder struct {
	net.TCPConn
	// TODO allow users to provide factory to create io.Writer per request/response pair
	writer io.Writer

	reqTee io.Writer
	resTee io.Writer

	reqID int
}

// prefix is prefixed to all responses
func NewRecorder(conn *net.TCPConn, prefix string) *Recorder {
	writer := os.Stdout
	resTee := NewTeeWriter(conn, os.Stdout, prefix)

	return &Recorder{
		TCPConn: *conn,
		writer:  writer,
		reqID:   0,

		resTee: resTee,
	}
}

var _ net.Conn = (*Recorder)(nil)

func (r *Recorder) ReadFrom(rdr io.Reader) (int64, error) {
	n, err := io.Copy(r.resTee, rdr)
	if err != nil {
		return n, err
	}
	return n, nil
}

// TODO implement WriteTo
func (r *Recorder) Write(b []byte) (int, error) {
	return r.reqTee.Write(b)
}

func (r *Recorder) Close() error {
	return nil
}
