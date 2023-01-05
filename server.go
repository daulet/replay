package replay

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

type proxy struct {
	port int
	rw   io.ReadWriteCloser

	log *zap.Logger
}

type ProxyOption func(*proxy)

func ProxyLogger(log *zap.Logger) ProxyOption {
	return func(p *proxy) {
		p.log = log
	}
}

func NewProxy(
	port int,
	readWriter io.ReadWriteCloser,
	opts ...ProxyOption,
) *proxy {
	p := &proxy{
		port: port,
		rw:   readWriter,

		log: zap.NewNop(),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *proxy) Serve(ctx context.Context) error {
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
			p.handle(src, p.rw)
			src.Close()
		}()
	}
}

func (p *proxy) handle(src io.ReadWriteCloser, dst io.ReadWriter) {
	go func() {
		if _, err := io.Copy(dst, src); err != nil {
			p.log.Error("write from in to out", zap.Error(err))
		}
	}()
	if _, err := io.Copy(src, dst); err != nil {
		p.log.Error("write from out to in", zap.Error(err))
	}
}
