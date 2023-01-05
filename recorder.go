package replay

import (
	"io"
	"net"
)

type Recorder struct {
	// TODO stop embedding
	net.TCPConn
	closed chan struct{}

	writer *Writer
	reqTee io.Writer
	resTee io.Writer
}

type FilenameFunc func(reqID int) string

func NewRecorder(addr string, reqFileFunc, respFileFunc FilenameFunc) (io.ReadWriteCloser, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	writer := NewWriter(reqFileFunc, respFileFunc)
	reqTee := writer.RequestWriter()
	resTee := writer.ResponseWriter()

	return &Recorder{
		TCPConn: *conn.(*net.TCPConn),
		closed:  make(chan struct{}),

		writer: writer,
		reqTee: reqTee,
		resTee: resTee,
	}, nil
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
	r.writer.Close()
	return r.TCPConn.Close()
}
