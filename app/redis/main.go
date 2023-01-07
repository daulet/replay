package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"

	"github.com/daulet/replay"
	"github.com/daulet/replay/redis"

	"go.uber.org/zap"
)

// $ go run app/main.go --port 8080 --target localhost:6379
// $ redis-cli -p 8080

func main() {
	var (
		ctx      = context.Background()
		err      error
		recorder io.ReadWriteCloser

		record = flag.Bool("record", false, "record traffic")
		port   = flag.Int("port", 0, "port to listen on")
		target = flag.String("target", "", "target host:port")

		reqFunc = func(reqID int) string {
			// TODO parametrize testdata dir
			return fmt.Sprintf("/testdata/%d.request", reqID)
		}
		resFunc = func(reqID int) string {
			return fmt.Sprintf("/testdata/%d.response", reqID)
		}
	)
	flag.Parse()

	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	if *record {
		recorder, err = replay.NewRecorder(*target, reqFunc, resFunc)
		if err != nil {
			panic(err)
		}
	} else {
		recorder, err = redis.NewReplayer(reqFunc, resFunc, redis.ReplayerLogger(logger))
		if err != nil {
			panic(err)
		}
	}
	defer recorder.Close()
	srv := replay.NewProxy(*port, recorder, replay.ProxyLogger(logger))
	if err := srv.Serve(ctx); err != nil {
		log.Fatal(err)
	}
}
