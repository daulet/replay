package redisreplay

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

type Mode int

const (
	ModeRecord Mode = iota + 1
	ModeReplay
)

type redisProxy struct {
	mode         Mode
	port         int
	remoteAddr   string
	reqFileFunc  FilenameFunc
	respFileFunc FilenameFunc

	log *zap.Logger
}

type ProxyOption func(*redisProxy)

func ProxyLogger(log *zap.Logger) ProxyOption {
	return func(p *redisProxy) {
		p.log = log
	}
}

func NewRedisProxy(
	mode Mode,
	port int,
	remoteAddr string,
	reqFileFunc FilenameFunc,
	respFileFunc FilenameFunc,
	opts ...ProxyOption,
) *redisProxy {
	p := &redisProxy{
		mode:         mode,
		port:         port,
		remoteAddr:   remoteAddr,
		reqFileFunc:  reqFileFunc,
		respFileFunc: respFileFunc,

		log: zap.NewNop(),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *redisProxy) Serve(ctx context.Context) error {
	var wg sync.WaitGroup
	defer wg.Wait()

	var lstr *net.TCPListener
	{
		cfg := &net.ListenConfig{Control: func(_, _ string, conn syscall.RawConn) error {
			return conn.Control(func(descriptor uintptr) {
				if err := syscall.SetsockoptInt(int(descriptor), syscall.SOL_SOCKET, unix.SO_REUSEPORT, 1); err != nil {
					return
				}
			})
		}}
		l, err := cfg.Listen(ctx, "tcp", fmt.Sprintf(":%d", p.port))
		if err != nil {
			return err
		}
		defer l.Close()
		lstr = l.(*net.TCPListener)
	}

	var rw io.ReadWriteCloser
	{
		switch p.mode {
		case ModeRecord:
			var remote *net.TCPConn
			{
				conn, err := net.Dial("tcp", p.remoteAddr)
				if err != nil {
					return err
				}
				defer conn.Close()
				remote = conn.(*net.TCPConn)
			}
			rw = NewRecorder(remote, p.reqFileFunc, p.respFileFunc)
		case ModeReplay:
			rep, err := NewReplayer(p.reqFileFunc, p.respFileFunc, ReplayerLogger(p.log))
			if err != nil {
				return err
			}
			rw = rep
		default:
			return fmt.Errorf("unknown mode: %d", p.mode)
		}
	}
	defer rw.Close()

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		lstr.SetDeadline(time.Now().Add(100 * time.Millisecond))
		src, err := lstr.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && !ne.Timeout() {
				p.log.Error("accept error", zap.Error(err))
			}
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.handle(src, rw)
			src.Close()
		}()
	}
}

func (p *redisProxy) handle(src io.ReadWriteCloser, dst io.ReadWriter) {
	go func() {
		if _, err := io.Copy(dst, src); err != nil {
			p.log.Error("write from in to out", zap.Error(err))
		}
	}()
	if _, err := io.Copy(src, dst); err != nil {
		p.log.Error("write from out to in", zap.Error(err))
	}
}
