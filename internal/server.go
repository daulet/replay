package internal

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

type Server struct {
	ready chan struct{}
	port  int
	rw    io.ReadWriteCloser

	log *zap.Logger
}

type ServerOption func(*Server)

func ServerLogger(log *zap.Logger) ServerOption {
	return func(p *Server) {
		p.log = log
	}
}

func NewServer(
	port int,
	readWriter io.ReadWriteCloser,
	opts ...ServerOption,
) *Server {
	p := &Server{
		ready: make(chan struct{}),
		port:  port,
		rw:    readWriter,

		log: zap.NewNop(),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *Server) Ready() <-chan struct{} {
	return p.ready
}

func (p *Server) Serve(ctx context.Context) error {
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

	close(p.ready)

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

func (p *Server) handle(src io.ReadWriteCloser, dst io.ReadWriter) {
	go func() {
		if _, err := io.Copy(dst, src); err != nil {
			p.log.Error("write from in to out", zap.Error(err))
		}
	}()
	if _, err := io.Copy(src, dst); err != nil {
		p.log.Error("write from out to in", zap.Error(err))
	}
}
