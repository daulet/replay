package redisreplay

import "io"

type lineTeeWriter struct {
	dst       io.Writer
	modDst    io.Writer
	modPrefix string
	newline   bool
}

var _ io.Writer = (*lineTeeWriter)(nil)

func NewTeeWriter(dst io.Writer, modDst io.Writer, prefix string) io.Writer {
	return &lineTeeWriter{
		dst:       dst,
		modDst:    modDst,
		modPrefix: prefix,
		newline:   true,
	}
}

func (lw *lineTeeWriter) Write(p []byte) (int, error) {
	n, err := lw.dst.Write(p)
	if err != nil {
		return n, err
	}
	for i, b := range p {
		if lw.newline {
			if _, err := lw.modDst.Write([]byte(lw.modPrefix)); err != nil {
				return i, err
			}
			lw.newline = false
		}
		if _, err := lw.modDst.Write([]byte{b}); err != nil {
			return i, err
		}
		lw.newline = b == '\n'
	}
	return len(p), nil
}
