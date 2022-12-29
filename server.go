package redisreplay

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
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
}

func NewRedisProxy(
	mode Mode,
	port int,
	remoteAddr string,
	reqFileFunc FilenameFunc,
	respFileFunc FilenameFunc,
) *redisProxy {
	return &redisProxy{
		mode:         mode,
		port:         port,
		remoteAddr:   remoteAddr,
		reqFileFunc:  reqFileFunc,
		respFileFunc: respFileFunc,
	}
}

func (p *redisProxy) Serve(ctx context.Context) error {
	var wg sync.WaitGroup
	defer wg.Wait()

	var lstr *net.TCPListener
	{
		l, err := net.Listen("tcp", fmt.Sprintf(":%d", p.port))
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
			rep, err := NewReplayer()
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
				log.Printf("accept error: %v", err)
			}
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			handle(src, rw)
			src.Close()
		}()
	}
}

// TODO add transparency logger, that just prints out all comms
func handle(src io.ReadWriteCloser, dst io.ReadWriter) {
	go func() {
		if _, err := io.Copy(dst, src); err != nil {
			log.Printf("write from in to out: %v", err)
		}
	}()
	if _, err := io.Copy(src, dst); err != nil {
		log.Printf("write from out to in: %v", err)
	}
}
