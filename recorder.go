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

	reqTee := NewTeeWriter(io.Discard, writer.RequestWriter(), "")
	resTee := NewTeeWriter(conn, writer.ResponseWriter(), "")

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
