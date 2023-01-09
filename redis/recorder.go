package redis

import (
	"io"
	"net"

	"github.com/daulet/replay"
	"github.com/daulet/replay/internal"
)

type recorder struct {
	conn net.Conn

	writer *internal.Writer
	reqTee io.Writer
	resTee io.Writer
}

func newRecorder(addr string, reqFileFunc, respFileFunc replay.FilenameFunc) (io.ReadWriteCloser, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	writer := internal.NewWriter(reqFileFunc, respFileFunc)
	reqTee := writer.RequestWriter()
	resTee := writer.ResponseWriter()

	return &recorder{
		conn: conn,

		writer: writer,
		reqTee: reqTee,
		resTee: resTee,
	}, nil
}

var _ io.ReadWriteCloser = (*recorder)(nil)

func (r *recorder) Read(p []byte) (int, error) {
	n, err := r.conn.Read(p)
	r.resTee.Write(p[:n])
	return n, err
}

func (r *recorder) ReadFrom(rdr io.Reader) (int64, error) {
	return io.Copy(r.conn, io.TeeReader(rdr, r.reqTee))
}

func (r *recorder) Write(p []byte) (int, error) {
	n, err := r.conn.Write(p)
	r.reqTee.Write(p[:n])
	return n, err
}

func (r *recorder) Close() error {
	r.conn.Close()
	r.writer.Close()
	return nil
}
