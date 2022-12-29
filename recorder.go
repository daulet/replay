package redisreplay

import (
	"io"
	"net"
)

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
	reqTee := writer.RequestWriter()
	resTee := writer.ResponseWriter()

	return &Recorder{
		TCPConn: *conn,
		writer:  writer,
		closed:  make(chan struct{}),

		reqTee: reqTee,
		resTee: resTee,
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

func (r *Recorder) ReadFrom(rdr io.Reader) (int64, error) {
	return io.Copy(&r.TCPConn, io.TeeReader(rdr, r.reqTee))
}

func (r *Recorder) Write(p []byte) (int, error) {
	select {
	case <-r.closed:
		return 0, io.EOF
	default:
	}
	r.reqTee.Write(p)
	return r.TCPConn.Write(p)
}

func (r *Recorder) Close() error {
	close(r.closed)
	return r.TCPConn.Close()
}
