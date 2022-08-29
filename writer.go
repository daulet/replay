package redisreplay

import (
	"fmt"
	"io"
	"os"
)

type funcWriter struct {
	write func(p []byte) (int, error)
}

var _ io.Writer = (*funcWriter)(nil)

func (w *funcWriter) Write(p []byte) (int, error) {
	return w.write(p)
}

type writer struct {
	// TODO allow users to provide factory to create io.Writer per request/response pair
	w io.Writer

	reqID           int
	writingResponse bool
	reqWriter       io.WriteCloser
	respWriter      io.WriteCloser
}

func NewWriter() *writer {
	return &writer{
		w:               os.Stdout,
		writingResponse: true,
	}
}

func (w *writer) RequestWriter() io.Writer {
	return &funcWriter{
		write: w.writeRequest,
	}
}

func (w *writer) ResponseWriter() io.Writer {
	return &funcWriter{
		write: w.writeResponse,
	}
}

func (w *writer) Close() error {
	if w.respWriter != nil {
		return w.respWriter.Close()
	}
	return nil
}

func (w *writer) writeRequest(p []byte) (int, error) {
	if w.reqWriter == nil {
		if w.respWriter != nil {
			w.respWriter.Close()
			w.respWriter = nil
		}

		f, err := os.Create(fmt.Sprintf("%d.request", w.reqID))
		if err != nil {
			return 0, err
		}
		w.reqWriter = f

		w.reqID++
	}
	return w.reqWriter.Write(p)
}

func (w *writer) writeResponse(p []byte) (int, error) {
	if w.respWriter == nil {
		if w.reqWriter != nil {
			w.reqWriter.Close()
			w.reqWriter = nil
		}

		f, err := os.Create(fmt.Sprintf("%d.response", w.reqID-1))
		if err != nil {
			return 0, err
		}
		w.respWriter = f
	}
	return w.respWriter.Write(p)
}
