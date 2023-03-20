package postgres

import (
	"context"
	"fmt"
	"io"

	"github.com/daulet/replay"
	"github.com/daulet/replay/internal"

	"go.uber.org/zap"
)

type mode int

const (
	modeUnknown mode = iota
	modeRecord
	modeReplay
)

type proxy struct {
	*internal.Server
	rw io.ReadWriteCloser

	// required
	mode       mode
	remoteAddr string // applicable iff mode == modeRecord

	// optional
	reqFileFunc  replay.FilenameFunc
	respFileFunc replay.FilenameFunc
	log          *zap.Logger
}

type ProxyOption func(*proxy)

func ProxyRecord(remoteAddr string) ProxyOption {
	return func(p *proxy) {
		p.mode = modeRecord
		p.remoteAddr = remoteAddr
	}
}

func ProxyReplay() ProxyOption {
	return func(p *proxy) {
		p.mode = modeReplay
	}
}

func SavedRequest(f replay.FilenameFunc) ProxyOption {
	return func(p *proxy) {
		p.reqFileFunc = f
	}
}

func SavedResponse(f replay.FilenameFunc) ProxyOption {
	return func(p *proxy) {
		p.respFileFunc = f
	}
}

func ProxyLogger(log *zap.Logger) ProxyOption {
	return func(p *proxy) {
		p.log = log
	}
}

func NewProxy(
	port int,
	opts ...ProxyOption,
) (*proxy, error) {
	p := &proxy{
		reqFileFunc: func(reqID int) string {
			return fmt.Sprintf("testdata/%d.request", reqID)
		},
		respFileFunc: func(reqID int) string {
			return fmt.Sprintf("testdata/%d.response", reqID)
		},
		log: zap.NewNop(), // optional parameter
	}
	for _, opt := range opts {
		opt(p)
	}

	var (
		rw  io.ReadWriteCloser
		err error
	)
	switch p.mode {
	case modeRecord:
		rw, err = newRecorder(p.log, p.remoteAddr, p.reqFileFunc, p.respFileFunc)
	case modeReplay:
		rw, err = newReplayer(p.log, p.reqFileFunc, p.respFileFunc)
	default:
		err = fmt.Errorf("unknown proxy mode. must set ProxyRecord or ProxyReplay option")
	}
	if err != nil {
		return nil, err
	}
	p.Server = internal.NewServer(port, rw, internal.ServerLogger(p.log))
	p.rw = rw
	return p, nil
}

func (p *proxy) Serve(ctx context.Context) error {
	defer p.rw.Close()
	return p.Server.Serve(ctx)
}
