package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"

	"github.com/daulet/replay"
	"github.com/daulet/replay/redis"
)

// $ go run app/main.go --port 8080 --target localhost:6379
// $ redis-cli -p 8080

func main() {
	var (
		ctx    = context.Background()
		record = flag.Bool("record", false, "record traffic")
		port   = flag.Int("port", 0, "port to listen on")
		target = flag.String("target", "", "target host:port")

		reqFunc = func(reqID int) string {
			return fmt.Sprintf("testdata/%d.request", reqID)
		}
		resFunc = func(reqID int) string {
			return fmt.Sprintf("testdata/%d.response", reqID)
		}
	)
	flag.Parse()

	lstr, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		panic(err)
	}
	defer lstr.Close()

	var recorder io.ReadWriteCloser
	if *record {
		recorder, err = replay.NewRecorder(*target, reqFunc, resFunc)
		if err != nil {
			panic(err)
		}
	} else {
		recorder, err = redis.NewReplayer(reqFunc, resFunc)
		if err != nil {
			panic(err)
		}
	}
	defer recorder.Close()
	srv := replay.NewProxy(*port, recorder)
	if err := srv.Serve(ctx); err != nil {
		log.Fatal(err)
	}
}
