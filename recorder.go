package redisreplay

import (
	"io"
	"net"
)

// hide the TCPConn methods we don't want to expose, e.g. ReadFrom
// TODO why did we do this? What's wrong with using ReadFrom?
type readWriteCloserOnly struct {
	io.ReadWriteCloser
}

type Recorder struct {
	net.TCPConn
	writer *writer
	closed chan struct{}

	reqTee io.Writer
	resTee io.Writer
}

type FilenameFunc func(reqID int) string

func NewRecorder(conn *net.TCPConn, reqFileFunc, respFileFunc FilenameFunc) io.ReadWriteCloser {
	writer := NewWriter(reqFileFunc, respFileFunc)

	// TODO there is no reason this shouldn't work on Postgres
	reqTee := NewTeeWriter(conn, writer.RequestWriter(), "")
	// TODO understand why this doesn't require conn
	resTee := NewTeeWriter(io.Discard, writer.ResponseWriter(), "")

	return readWriteCloserOnly{
		&Recorder{
			TCPConn: *conn,
			writer:  writer,
			closed:  make(chan struct{}),

			reqTee: reqTee,
			resTee: resTee,
		},
	}
}

var _ net.Conn = (*Recorder)(nil)

func (r *Recorder) Read(p []byte) (int, error) {
	select {
	case <-r.closed:
		return 0, io.EOF
	default:
	}
	n, err := r.TCPConn.Read(p)
	r.resTee.Write(p[:n])
	return n, err
}

func (r *Recorder) Write(p []byte) (int, error) {
	select {
	case <-r.closed:
		return 0, io.EOF
	default:
	}
	// TODO why we never write to r.TCPConn?
	return r.reqTee.Write(p)
}

func (r *Recorder) Close() error {
	close(r.closed)
	return r.TCPConn.Close()
}
