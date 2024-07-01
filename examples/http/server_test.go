package main_test

import (
	"context"
	"flag"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	example "examples/http"

	"github.com/daulet/replay"
)

var update = flag.Bool("update", false, "update golden files")

func TestServe(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := example.Serve(ctx, 8080); err != nil {
			t.Error(err)
		}
	}()

	srv, err := replay.NewHTTPServer(8081, *update, "localhost:8080", "http.record")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		url      string
		wantBody string
	}{
		{
			name:     "foo",
			url:      "http://localhost:8081/foo",
			wantBody: "Hello, \"/foo\"",
		},
		{
			name:     "bar",
			url:      "http://localhost:8081/bar",
			wantBody: "Hi, \"/bar\"",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resp, err := http.Get(test.url)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()

			if string(body) != test.wantBody {
				t.Errorf("got %q, want %q", body, test.wantBody)
			}
		})

	}

	cancel()
	srv.Close()
	wg.Wait()
}

func TestMain(m *testing.M) {
	flag.Parse()
	m.Run()
}
