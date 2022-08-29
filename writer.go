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
	// TODO do we need to close a file?
	return nil
}

func (w *writer) writeRequest(p []byte) (int, error) {
	if w.writingResponse {
		w.writingResponse = false

		w.w.Write([]byte("--------\n"))
		w.w.Write([]byte(fmt.Sprintf("request %d\n", w.reqID)))
		w.w.Write([]byte("--------\n"))

		w.reqID++
	}
	return w.w.Write(p)
}

func (w *writer) writeResponse(p []byte) (int, error) {
	w.writingResponse = true
	return w.w.Write(p)
}
