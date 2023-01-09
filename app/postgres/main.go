package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/daulet/replay/postgres"

	"go.uber.org/zap"
)

// $ go run app/postgres/main.go --port 8080 --target localhost:5432
// $ psql -h localhost -p 8080 -U testuser testdb

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	var (
		ctx = context.Background()

		replay = flag.Bool("replay", false, "record traffic")
		port   = flag.Int("port", 0, "port to listen on")
		target = flag.String("target", "", "target host:port")

		opts = []postgres.ProxyOption{
			postgres.SavedRequest(func(reqID int) string {
				// TODO parametrize testdata dir
				return fmt.Sprintf("/testdata/%d.request", reqID)
			}),
			postgres.SavedResponse(func(reqID int) string {
				return fmt.Sprintf("/testdata/%d.response", reqID)
			}),
			postgres.ProxyLogger(logger),
		}
	)
	flag.Parse()

	reOpt := postgres.ProxyReplay()
	if !*replay {
		reOpt = postgres.ProxyRecord(*target)
	}
	opts = append(opts, reOpt)

	srv, err := postgres.NewProxy(*port, opts...)
	if err != nil {
		log.Fatal(err)
	}
	if err := srv.Serve(ctx); err != nil {
		log.Fatal(err)
	}
}
