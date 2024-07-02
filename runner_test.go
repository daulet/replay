//go:build !cli.test

package replay_test

import (
	"flag"
	"os"
	"os/exec"
	"testing"

	cmdtest "github.com/google/go-cmdtest"
)

var update = flag.Bool("update", false, "update test files with results")

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}

func TestCLI(t *testing.T) {
	if err := exec.Command("go", "test", "-c", "-tags", "cli.test", ".").Run(); err != nil {
		t.Fatal(err)
	}
	defer os.Remove("replay.test")

	ts, err := cmdtest.Read("testdata/runner")
	if err != nil {
		t.Fatal(err)
	}
	ts.Commands["replay.test"] = cmdtest.Program("replay.test")
	ts.Run(t, *update)
}
