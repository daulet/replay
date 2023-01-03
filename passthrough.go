package replay

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

type passthrough struct {
}

func NewPassthrough() *passthrough {
	return &passthrough{}
}

func (p *passthrough) Serve(ctx context.Context, port int, remoteAddr string, in, out io.Writer) error {
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
		l, err := cfg.Listen(ctx, "tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			return err
		}
		defer l.Close()
		lstr = l.(*net.TCPListener)
	}

	timeout := 100 * time.Millisecond
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		lstr.SetDeadline(time.Now().Add(timeout))
		src, err := lstr.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && !ne.Timeout() {
				// TODO consistent logger
				log.Printf("accept error: %v", err)
			}
			continue
		}

		var dst *net.TCPConn
		{
			conn, err := net.DialTimeout("tcp", remoteAddr, timeout)
			if err != nil {
				return err
			}
			dst = conn.(*net.TCPConn)
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			io.Copy(dst, io.TeeReader(src, in))
			dst.Close()
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			io.Copy(src, io.TeeReader(dst, out))
			src.Close()
		}()
	}
}
