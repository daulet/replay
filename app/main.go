package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"

	"github.com/daulet/replay"
)

// $ go run app/main.go --port 8080 --target localhost:6379
// $ redis-cli -p 8080

func main() {
	var (
		ctx    = context.Background()
		record = flag.Bool("record", false, "record traffic")
		port   = flag.Int("port", 0, "port to listen on")
		target = flag.String("target", "", "target host:port")
	)
	flag.Parse()

	lstr, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		panic(err)
	}
	defer lstr.Close()

	mode := replay.ModeReplay
	if *record {
		mode = replay.ModeRecord
	}
	srv := replay.NewRedisProxy(mode, *port, *target,
		func(reqID int) string {
			return fmt.Sprintf("testdata/%d.request", reqID)
		},
		func(reqID int) string {
			return fmt.Sprintf("testdata/%d.response", reqID)
		},
	)
	if err := srv.Serve(ctx); err != nil {
		log.Fatal(err)
	}
}
