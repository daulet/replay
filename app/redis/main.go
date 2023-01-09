package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/daulet/replay/redis"

	"go.uber.org/zap"
)

// $ go run app/main.go --port 8080 --target localhost:6379
// $ redis-cli -p 8080

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	var (
		ctx = context.Background()

		record = flag.Bool("record", false, "record traffic")
		port   = flag.Int("port", 0, "port to listen on")
		target = flag.String("target", "", "target host:port")

		opts = []redis.ProxyOption{
			redis.SavedRequest(func(reqID int) string {
				// TODO parametrize testdata dir
				return fmt.Sprintf("/testdata/%d.request", reqID)
			}),
			redis.SavedResponse(func(reqID int) string {
				return fmt.Sprintf("/testdata/%d.response", reqID)
			}),
			redis.ProxyLogger(logger),
		}
	)
	flag.Parse()

	reOpt := redis.ProxyReplay()
	if *record {
		reOpt = redis.ProxyRecord(*target)
	}
	opts = append(opts, reOpt)

	srv, err := redis.NewProxy(*port, opts...)
	if err != nil {
		log.Fatal(err)
	}
	if err := srv.Serve(ctx); err != nil {
		log.Fatal(err)
	}
}
