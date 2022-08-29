package redisreplay

import (
	"io"
	"net"
)

// hide the TCPConn methods we don't want to expose, e.g. ReadFrom
type readWriteCloserOnly struct {
	io.ReadWriteCloser
}

type Recorder struct {
	net.TCPConn
	writer *writer

	reqTee io.Writer
	resTee io.Writer
}

type FilenameFunc func(reqID int) string

func NewRecorder(conn *net.TCPConn, reqFileFunc, respFileFunc FilenameFunc) io.ReadWriteCloser {
	writer := NewWriter(reqFileFunc, respFileFunc)

	// TODO there is no reason this shouldn't work on Postgres
	reqTee := NewTeeWriter(conn, writer.RequestWriter(), "")
	resTee := NewTeeWriter(io.Discard, writer.ResponseWriter(), "")

	return readWriteCloserOnly{
		&Recorder{
			TCPConn: *conn,
			writer:  writer,

			reqTee: reqTee,
			resTee: resTee,
		},
	}
}

var _ net.Conn = (*Recorder)(nil)

func (r *Recorder) ReadFrom(rdr io.Reader) (int64, error) {
	return io.Copy(r.resTee, rdr)
}

func (r *Recorder) Read(p []byte) (int, error) {
	n, err := r.TCPConn.Read(p)
	r.resTee.Write(p[:n])
	return n, err
}

func (r *Recorder) Write(p []byte) (int, error) {
	return r.reqTee.Write(p)
}

func (r *Recorder) Close() error {
	return r.TCPConn.Close()
}
