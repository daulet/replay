package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/daulet/replay"
	"github.com/daulet/replay/internal"

	"go.uber.org/zap"
)

// This is great for capturing new protocol, to disassemble it
func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	var (
		ctx = context.Background()

		port   = flag.Int("port", 0, "port to listen on")
		target = flag.String("target", "", "target host:port")
	)
	flag.Parse()

	w := internal.NewWriter(
		func(reqID int) string {
			return fmt.Sprintf("./testdata/%d.request", reqID)
		},
		func(reqID int) string {
			return fmt.Sprintf("./testdata/%d.response", reqID)
		},
	)
	defer w.Close()

	srv := replay.NewPassthrough()
	if err := srv.Serve(ctx, *port, *target, w.RequestWriter(), w.ResponseWriter()); err != nil {
		log.Fatal(err)
	}
}
