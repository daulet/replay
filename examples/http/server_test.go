package main_test

import (
	"context"
	example "examples/http"
	"sync"
	"testing"
)

func TestServe(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := example.Serve(ctx, 8080); err != nil {
			t.Fatal(err)
		}
	}()

	cancel()
	wg.Wait()
}
