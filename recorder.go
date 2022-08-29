package redisreplay

import (
	"io"
	"io/ioutil"
	"net"
	"os"
)

// hide the TCPConn methods we don't want to expose, e.g. ReadFrom
type readWriteCloserOnly struct {
	io.ReadWriteCloser
}

// TODO need to differentiate between request and response callers
type Recorder struct {
	net.TCPConn
	// TODO allow users to provide factory to create io.Writer per request/response pair
	writer io.Writer

	reqTee io.Writer
	resTee io.Writer

	reqID int
}

func NewRecorder(conn *net.TCPConn) io.ReadWriteCloser {
	writer := os.Stdout
	reqTee := NewTeeWriter(ioutil.Discard, writer, "request: ")
	resTee := NewTeeWriter(conn, writer, "response: ")

	return readWriteCloserOnly{
		&Recorder{
			TCPConn: *conn,
			writer:  writer,
			reqID:   0,

			reqTee: reqTee,
			resTee: resTee,
		},
	}
}

var _ net.Conn = (*Recorder)(nil)

func (r *Recorder) ReadFrom(rdr io.Reader) (int64, error) {
	return io.Copy(r.reqTee, rdr)
}

func (r *Recorder) Read(p []byte) (int, error) {
	n, err := r.TCPConn.Read(p)
	r.reqTee.Write(p[:n])
	return n, err
}

func (r *Recorder) Write(p []byte) (int, error) {
	return r.resTee.Write(p)
}

func (r *Recorder) Close() error {
	return r.TCPConn.Close()
}
