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
	record := flag.Bool("record", false, "record traffic")
	flag.Parse()

	lstr, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		panic(err)
	}
	defer lstr.Close()

	var dst io.ReadWriter
	if *record {
		remote, err := net.Dial("tcp", *target)
		if err != nil {
			panic(err)
		}
		defer remote.Close()

		// TODO this should be a distinct service
		dst = redisreplay.NewRecorder(
			remote.(*net.TCPConn),
			func(reqID int) string {
				return fmt.Sprintf("testdata/%d.request", reqID)
			},
			func(reqID int) string {
				return fmt.Sprintf("testdata/%d.response", reqID)
			},
		)
	} else {
		dst, err = redisreplay.NewReplayer()
		if err != nil {
			panic(err)
		}
	}

	for {
		src, err := lstr.Accept()
		if err != nil {
			panic(err)
		}
		go handle(src, dst)
	}
}

// TODO add transparency logger, that just prints out all comms
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
