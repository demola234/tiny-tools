package cli_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/demola234/tiny-tools/mock_machina/internal/cli"
)

func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"mockmachina": func() {
			os.Exit(cli.Run(context.Background(), cli.Env{
				Args:   os.Args[1:],
				Stdout: os.Stdout,
				Stderr: os.Stderr,
				Info:   testInfo,
			}))
		},
	})
}

func TestScripts(t *testing.T) {
	t.Parallel()
	action, err := filepath.Abs(filepath.Join("..", "..", "action", "run.sh"))
	if err != nil {
		t.Fatal(err)
	}
	testscript.Run(t, testscript.Params{
		Dir: filepath.Join("..", "..", "testdata", "script"),
		Setup: func(env *testscript.Env) error {
			env.Setenv("ACTION_SH", action)
			return nil
		},
	})
}
