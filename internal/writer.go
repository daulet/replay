package internal

import (
	"io"
	"os"
	"path"
	"sync"

	"github.com/daulet/replay"
)

type funcWriter struct {
	mux   *sync.Mutex
	write func(p []byte) (int, error)
}

var _ io.Writer = (*funcWriter)(nil)

func (w *funcWriter) Write(p []byte) (int, error) {
	w.mux.Lock()
	defer w.mux.Unlock()
	return w.write(p)
}

type Writer struct {
	reqFilenameFunc  replay.FilenameFunc
	respFilenameFunc replay.FilenameFunc

	mux        sync.Mutex
	reqID      int
	reqWriter  io.WriteCloser
	respWriter io.WriteCloser
}

func NewWriter(reqFilenameFunc, respFilenameFunc replay.FilenameFunc) *Writer {
	return &Writer{
		reqFilenameFunc:  reqFilenameFunc,
		respFilenameFunc: respFilenameFunc,
	}
}

func (w *Writer) RequestWriter() io.Writer {
	return &funcWriter{
		mux:   &w.mux,
		write: w.writeRequest,
	}
}

func (w *Writer) ResponseWriter() io.Writer {
	return &funcWriter{
		mux:   &w.mux,
		write: w.writeResponse,
	}
}

func (w *Writer) Close() error {
	if w.respWriter != nil {
		return w.respWriter.Close()
	}
	return nil
}

func (w *Writer) writeRequest(p []byte) (int, error) {
	if w.reqWriter != nil {
		return w.reqWriter.Write(p)
	}
	if w.respWriter != nil {
		w.respWriter.Close()
		w.respWriter = nil
	}
	fname := w.reqFilenameFunc(w.reqID)
	if err := os.MkdirAll(path.Dir(fname), os.ModePerm); err != nil {
		return 0, err
	}
	f, err := os.Create(fname)
	if err != nil {
		return 0, err
	}
	w.reqWriter = f
	w.reqID++
	return w.reqWriter.Write(p)
}

func (w *Writer) writeResponse(p []byte) (int, error) {
	if w.respWriter != nil {
		return w.respWriter.Write(p)
	}
	if w.reqWriter != nil {
		w.reqWriter.Close()
		w.reqWriter = nil
	}
	fname := w.respFilenameFunc(w.reqID - 1)
	if err := os.MkdirAll(path.Dir(fname), os.ModePerm); err != nil {
		return 0, err
	}
	f, err := os.Create(fname)
	if err != nil {
		return 0, err
	}
	w.respWriter = f
	return w.respWriter.Write(p)
}
