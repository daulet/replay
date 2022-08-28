package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
)

// $ go run main.go --port 8080 --dest localhost:6379
// $ redis-cli -p 8080

func main() {
	dest := flag.String("dest", "", "destination host:port")
	port := flag.Int("port", 0, "port to listen on")
	flag.Parse()

	target, err := net.Dial("tcp", *dest)
	if err != nil {
		panic(err)
	}
	defer target.Close()

	lstr, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		panic(err)
	}
	defer lstr.Close()

	for {
		conn, err := lstr.Accept()
		if err != nil {
			panic(err)
		}
		go handle(conn, target)
	}
}

func handle(in net.Conn, out net.Conn) {
	defer in.Close()

	reqTee := NewTeedWriter(out, os.Stdout, "request: ")

	resTee := NewTeedWriter(in, os.Stdout, "response: ")

	go func() {
		if _, err := io.Copy(reqTee, in); err != nil {
			log.Printf("write from in to out: %v", err)
		}
	}()
	if _, err := io.Copy(resTee, out); err != nil {
		log.Printf("write from out to in: %v", err)
	}
}

type lineTeeWriter struct {
	dst       io.Writer
	modDst    io.Writer
	modPrefix string
	newline   bool
}

var _ io.Writer = (*lineTeeWriter)(nil)

func NewTeedWriter(dst io.Writer, modDst io.Writer, prefix string) io.Writer {
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
