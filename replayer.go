package redisreplay

import (
	"bytes"
	"io"
)

// Replayer acts like a connection to remote redis server
type Replayer struct {
	buffer  *bytes.Buffer
	matcher *matcher
}

func NewReplayer() (io.ReadWriteCloser, error) {
	matcher, err := NewMatcher()
	if err != nil {
		return nil, err
	}
	return readWriteCloserOnly{
		&Replayer{
			buffer:  &bytes.Buffer{},
			matcher: matcher,
		},
	}, nil
}

func (r *Replayer) Read(p []byte) (int, error) {
	return r.matcher.Read(p)
}

func (r *Replayer) Write(p []byte) (int, error) {
	for _, b := range p {
		r.buffer.WriteByte(b)
		// TODO maybe this belongs in matcher
		// TODO see ReadBytes(delim byte) (line []byte, err error) for replacement
		if b == '\n' {
			line := r.buffer.Bytes()
			r.matcher.WriteLine(line)
			r.buffer.Reset()
		}
	}
	return len(p), nil
}

func (r *Replayer) Close() error {
	return nil
}
