package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"

	"github.com/daulet/redisreplay"
)

// $ go run app/main.go --port 8080 --dest localhost:6379
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

	reqTee := redisreplay.NewTeeWriter(out, os.Stdout, "request: ")
	resTee := redisreplay.NewTeeWriter(in, os.Stdout, "response: ")

	go func() {
		if _, err := io.Copy(reqTee, in); err != nil {
			log.Printf("write from in to out: %v", err)
		}
	}()
	if _, err := io.Copy(resTee, out); err != nil {
		log.Printf("write from out to in: %v", err)
	}
}
