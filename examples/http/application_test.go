package main_test

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	example "examples/http"

	"github.com/daulet/replay"
)

var (
	create         = flag.Bool("create", false, "create (record) test case")
	createTestName = flag.String("test_name", "newtest", "name of the test case to create")
	update         = flag.Bool("update", false, "update recordings for existing test cases")
)

func TestMain(m *testing.M) {
	flag.Parse()
	m.Run()
}

func TestApplicationTableDriven(t *testing.T) {
	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	const testdataDir = "testdata/application"
	// create if not exists
	_ = os.Mkdir(testdataDir, 0755)

	// start the dependency server if necessary, i.e. if recording
	if *update {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := example.ServeDependency(ctx, 8082); err != nil {
				t.Error(err)
			}
		}()
	}

	// start the server under test
	// since it's blocking call, we run it in a goroutine
	// and wait for it to finish with wg.Wait() below
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Note the application points to the record/replay port, not the dependency service directly.
		// This is the only modification needed to make the application testable - swap out address of dependency service.
		if err := example.ServeApplication(ctx, 8080, "localhost:8081"); err != nil {
			t.Error(err)
		}
	}()

	// Start the record/replay server, controlled by the -update flag.
	// In replay mode (default), it reads responses from the record file and never sends requests to the dependency service (port 8082).
	// In record/update mode, it sends requests to the dependency service and records responses to the specified file, which later could
	// be used in replay mode.
	srv, err := replay.NewHTTPServer(8081, *update, "localhost:8082", filepath.Join(testdataDir, "http.record"))
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
			url:      "http://localhost:8080/foo",
			wantBody: "Hello, \"/foo/\"",
		},
		{
			name:     "foo/5",
			url:      "http://localhost:8080/foo/5",
			wantBody: "Hello, \"/foo/25\"",
		},
		{
			name:     "bar",
			url:      "http://localhost:8080/bar",
			wantBody: "Hi, \"/bar/\"",
		},
		{
			name:     "bar/7",
			url:      "http://localhost:8080/bar/7",
			wantBody: "Hi, \"/bar/49\"",
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
	// 1. cancel the context to stop application and dependency services.
	// 2. close the record/replay server.
	// 3. wait for all services to return to guarantee corresponding ports are released,
	// 	  which is important for testing with -count=X.
	cancel()
	srv.Close()
	wg.Wait()
}

// testCases returns a list of test cases in the provided test data directory.
// Each subdirectory is considered a test case, each of which could store
// multiple recording files (for example, for different dependencies).
// If -create flag is set, it creates a corresponding subdirectory to store the recordings,
// and only returns that test case.
func testCases(testdataDir string) ([]string, error) {
	if *create {
		// ignore if already exists
		_ = os.Mkdir(fmt.Sprintf("%s/%s", testdataDir, *createTestName), 0o755)
		return []string{*createTestName}, nil
	}
	files, err := os.ReadDir(testdataDir)
	if err != nil {
		return nil, err
	}
	var cases []string
	for _, testDir := range files {
		if !testDir.IsDir() {
			continue
		}
		cases = append(cases, testDir.Name())
	}
	return cases, nil
}

// TODO simpler version of this test that doesn't require a dependency
func TestApplicationWithRunner(t *testing.T) {
	const testdataDir = "testdata/runner"
	// create if not exists
	_ = os.Mkdir(testdataDir, 0755)

	testCases, err := testCases(testdataDir)
	if err != nil {
		t.Fatal(err)
	}

	for _, testCase := range testCases {
		t.Run(testCase, func(t *testing.T) {
			var (
				wg     sync.WaitGroup
				ctx    context.Context
				cancel context.CancelFunc
			)
			if *create {
				ctx, cancel = context.WithCancel(context.Background())
			} else {
				ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
			}

			testDir := filepath.Join(testdataDir, testCase)

			// start the dependency server if necessary, i.e. if recording
			if *create || *update {
				wg.Add(1)
				go func() {
					defer wg.Done()
					if err := example.ServeDependency(ctx, 8082); err != nil {
						t.Error(err)
					}
				}()
			}

			// start the server under test
			// since it's blocking call, we run it in a goroutine
			// and wait for it to finish with wg.Wait() below
			wg.Add(1)
			go func() {
				defer wg.Done()
				// Note the application points to the record/replay port, not the dependency service directly.
				// This is the only modification needed to make the application testable - swap out address of dependency service.
				if err := example.ServeApplication(ctx, 8080, "localhost:8081"); err != nil {
					t.Error(err)
				}
			}()

			srv, err := replay.NewHTTPServer(8081, *create || *update, "localhost:8082", filepath.Join(testDir, "http.record"))
			if err != nil {
				t.Fatal(err)
			}

			runner, err := replay.NewHTTPRunner(8079, "localhost:8080", testDir)
			if err != nil {
				t.Fatal(err)
			}
			switch {
			case *create:
				err = runner.Serve()
			default:
				err = runner.Replay(*update)
			}
			if err != nil {
				t.Error(err)
			}

			cancel()
			srv.Close()
			wg.Wait()
		})
	}
}
