package replay

import (
	"io"
	"os"
	"path"
	"sync"
)

type funcWriter struct {
	write func(p []byte) (int, error)
}

var _ io.Writer = (*funcWriter)(nil)

func (w *funcWriter) Write(p []byte) (int, error) {
	return w.write(p)
}

type Writer struct {
	reqFilenameFunc  FilenameFunc
	respFilenameFunc FilenameFunc

	writeMux   sync.Mutex
	reqID      int
	reqWriter  io.WriteCloser
	respWriter io.WriteCloser
}

func NewWriter(reqFilenameFunc, respFilenameFunc FilenameFunc) *Writer {
	return &Writer{
		reqFilenameFunc:  reqFilenameFunc,
		respFilenameFunc: respFilenameFunc,
	}
}

func (w *Writer) RequestWriter() io.Writer {
	return &funcWriter{
		write: w.writeRequest,
	}
}

func (w *Writer) ResponseWriter() io.Writer {
	return &funcWriter{
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
	w.writeMux.Lock()
	defer w.writeMux.Unlock()
	if w.reqWriter == nil {
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
	}
	return w.reqWriter.Write(p)
}

func (w *Writer) writeResponse(p []byte) (int, error) {
	w.writeMux.Lock()
	defer w.writeMux.Unlock()
	if w.respWriter == nil {
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
	}
	return w.respWriter.Write(p)
}
