package redisreplay

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

type redisProxy struct {
	port       int
	remoteAddr string
}

func NewRedisProxy(port int, remoteAddr string) *redisProxy {
	return &redisProxy{
		port:       port,
		remoteAddr: remoteAddr,
	}
}

func (p *redisProxy) Serve(ctx context.Context) error {
	var lstr *net.TCPListener
	{
		l, err := net.Listen("tcp", fmt.Sprintf(":%d", p.port))
		if err != nil {
			return err
		}
		defer l.Close()
		lstr = l.(*net.TCPListener)
	}
	var remote *net.TCPConn
	{
		conn, err := net.Dial("tcp", p.remoteAddr)
		if err != nil {
			return err
		}
		defer conn.Close()
		remote = conn.(*net.TCPConn)
	}
	rec := NewRecorder(
		remote,
		// TODO make configurable
		func(reqID int) string {
			return fmt.Sprintf("testdata/%d.request", reqID)
		},
		func(reqID int) string {
			return fmt.Sprintf("testdata/%d.response", reqID)
		},
	)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		lstr.SetDeadline(time.Now().Add(1 * time.Second))
		src, err := lstr.Accept()
		if err != nil {
			if ne, ok := err.(net.Error); ok && !ne.Timeout() {
				log.Printf("accept error: %v", err)
			}
			continue
		}
		go handle(src, rec)
	}
}

func handle(src io.ReadWriteCloser, dst io.ReadWriter) {
	defer src.Close()

	go func() {
		if _, err := io.Copy(dst, src); err != nil {
			log.Printf("write from in to out: %v", err)
		}
	}()
	if _, err := io.Copy(src, dst); err != nil {
		log.Printf("write from out to in: %v", err)
	}
}
