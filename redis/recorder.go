package redis

import (
	"io"
	"net"

	"github.com/daulet/replay"
	"github.com/daulet/replay/internal"
)

type Recorder struct {
	conn net.Conn

	writer *internal.Writer
	reqTee io.Writer
	resTee io.Writer
}

// TODO unexport
func NewRecorder(addr string, reqFileFunc, respFileFunc replay.FilenameFunc) (io.ReadWriteCloser, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	writer := internal.NewWriter(reqFileFunc, respFileFunc)
	reqTee := writer.RequestWriter()
	resTee := writer.ResponseWriter()

	return &Recorder{
		conn: conn,

		writer: writer,
		reqTee: reqTee,
		resTee: resTee,
	}, nil
}

var _ io.ReadWriteCloser = (*Recorder)(nil)

func (r *Recorder) Read(p []byte) (int, error) {
	n, err := r.conn.Read(p)
	r.resTee.Write(p[:n])
	return n, err
}

func (r *Recorder) ReadFrom(rdr io.Reader) (int64, error) {
	return io.Copy(r.conn, io.TeeReader(rdr, r.reqTee))
}

func (r *Recorder) Write(p []byte) (int, error) {
	n, err := r.conn.Write(p)
	r.reqTee.Write(p[:n])
	return n, err
}

func (r *Recorder) Close() error {
	r.conn.Close()
	r.writer.Close()
	return nil
}
