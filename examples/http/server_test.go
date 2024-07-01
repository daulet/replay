package main_test

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	example "examples/http"

	"github.com/daulet/replay"
)

const testdataDir = "testdata"

var update = flag.Bool("update", false, "update golden files")

func TestServe(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	// start the server under test
	// since it's blocking call, we run it in a goroutine
	// and wait for it to finish with wg.Wait() below
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := example.Serve(ctx, 8080); err != nil {
			t.Error(err)
		}
	}()

	// Start the record/replay server, controlled by the -update flag.
	// In replay mode (default), it reads responses from the record file and never sends requests to the server under test (port 8080).
	// In record/update mode, it sends requests to the server under test and records responses to the record file, which later could
	// be used in replay mode.
	srv, err := replay.NewHTTPServer(8081, *update, "localhost:8080", fmt.Sprintf("%s/%s", testdataDir, "http.record"))
	if err != nil {
		t.Fatal(err)
	}

	// Note: URLs point to record/replay server, and it will either replay response or forward the request to the server under test.
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

	// order matters here:
	// 1. cancel the context to stop the server under test.
	// 2. close the record/replay server.
	// 3. wait for the server under test to return to guarantee the port is released.
	cancel()
	srv.Close()
	wg.Wait()
}

func TestMain(m *testing.M) {
	flag.Parse()
	// create if not exists
	_ = os.Mkdir(testdataDir, 0755)
	m.Run()
}
