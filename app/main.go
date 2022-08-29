package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"

	"github.com/daulet/redisreplay"
)

// $ go run app/main.go --port 8080 --target localhost:6379
// $ redis-cli -p 8080

func main() {
	target := flag.String("target", "", "target host:port")
	port := flag.Int("port", 0, "port to listen on")
	flag.Parse()

	dst, err := net.Dial("tcp", *target)
	if err != nil {
		panic(err)
	}
	defer dst.Close()

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
		src := redisreplay.NewRecorder(conn.(*net.TCPConn))
		go handle(src, dst)
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
