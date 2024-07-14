package replay_test

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

func findTestdataDir(t *testing.T, relDir string) string {
	t.Helper()
	// this is complicated to support running this test in two different ways:
	// 1. go test -tags cli.test
	// 2. go test
	// The second mode builds the test binary and runs from different working directory.
	testdataDir := relDir
	if _, err := os.Stat(testdataDir); err != nil {
		ex, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		testdataDir = filepath.Join(filepath.Dir(ex), relDir)
	}
	return testdataDir
}

func testCases(testdataDir string) ([]string, error) {
	if *create {
		// ignore if already exists
		_ = os.Mkdir(fmt.Sprintf("%s/%s", testdataDir, *createTestName), 0o755)
		return []string{*createTestName}, nil
	}
	files, err := os.ReadDir(testdataDir)
	if err != nil {
		wd, _ := os.Getwd()
		return nil, fmt.Errorf("unable to read directory (current directory: %q): [%w]", wd, err)
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

func TestRunner(t *testing.T) {
	testdataDir := findTestdataDir(t, "testdata/app")

	// create if not exists
	_ = os.Mkdir(testdataDir, 0755)

	testCases, err := testCases(testdataDir)
	if err != nil {
		t.Fatal(err)
	}

	for _, testCase := range testCases {
		t.Run(testCase, func(t *testing.T) {
			var wg sync.WaitGroup
			ctx, cancel := context.WithCancel(context.Background())

			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := serve(ctx, 8080); err != nil {
					t.Error(err)
				}
			}()

			testDir := filepath.Join(testdataDir, testCase)
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
			wg.Wait()
		})
	}
}

func TestUnreachable(t *testing.T) {
	testdataDir := findTestdataDir(t, "testdata/unavailable")

	// create if not exists
	_ = os.Mkdir(testdataDir, 0755)

	testCases, err := testCases(testdataDir)
	if err != nil {
		t.Fatal(err)
	}

	for _, testCase := range testCases {
		t.Run(testCase, func(t *testing.T) {
			testDir := filepath.Join(testdataDir, testCase)
			runner, err := replay.NewHTTPRunner(8079, "localhost:1234", testDir)
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
		})
	}
}

func TestUpdateUnavailable(t *testing.T) {
	testdataDir := findTestdataDir(t, "testdata/unavailable")
	// TODO harcoded test case, can we do better?
	testDir := filepath.Join(testdataDir, "foo")
	runner, err := replay.NewHTTPRunner(8079, "localhost:1234", testDir)
	if err != nil {
		t.Error(err)
	}
	err = runner.Replay(true)
	if err != nil {
		t.Error(err)
	}
}

func TestRecordUnavailable(t *testing.T) {
	testdataDir := findTestdataDir(t, "testdata/unavailable")
	// TODO harcoded test case, can we do better?
	testDir := filepath.Join(testdataDir, "foo")
	runner, err := replay.NewHTTPRunner(8079, "localhost:1234", testDir)
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		err = runner.Serve()
		if err != nil {
			t.Error(err)
		}
	}()

	<-runner.Ready()

	// make http request
	resp, err := http.Get("http://localhost:8079/foo")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	// read the body, the contents don't matter as that will be asserted by diff in recorded responses
	_, err = io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	_, err = http.Get("http://localhost:8079/stop")
	if err != nil {
		t.Fatal(err)
	}
	wg.Wait()
}

// TODO add a test with expected diff so we can validate via runner_test

func serve(ctx context.Context, port int) error {
	mux := http.NewServeMux()
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}

	mux.HandleFunc("/foo", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "foo")
	})

	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background())
	}()

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}
