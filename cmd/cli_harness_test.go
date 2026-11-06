package cmd

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/swalha1999/lazycron/config"
	"github.com/swalha1999/lazycron/cron"
)

// cliResult is the outcome of one CLI invocation in tests.
type cliResult struct {
	code   int
	stdout string
	stderr string
}

// decode unmarshals stdout as JSON into v.
func (r cliResult) decode(t *testing.T, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(r.stdout), v); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, r.stdout)
	}
}

// errorKind returns error.kind from a JSON error on stderr.
func (r cliResult) errorKind(t *testing.T) string {
	t.Helper()
	var e struct {
		Error cliError `json:"error"`
	}
	if err := json.Unmarshal([]byte(r.stderr), &e); err != nil {
		t.Fatalf("stderr is not a JSON error: %v\n%s", err, r.stderr)
	}
	return e.Error.Kind
}

// testEnv is an isolated HOME plus a crontab file for CLI tests.
type testEnv struct {
	t        *testing.T
	home     string
	cronFile string
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Cleanup(func() { cron.DefaultRunner = config.RunnerPlain })
	return &testEnv{t: t, home: home, cronFile: filepath.Join(home, "crontab")}
}

// run executes lazycron with args against the env's crontab file.
func (e *testEnv) run(args ...string) cliResult {
	e.t.Helper()
	return runCLI(e.t, append([]string{"--cron-file", e.cronFile}, args...)...)
}

// mustRun is run that fails the test on a non-zero exit.
func (e *testEnv) mustRun(args ...string) cliResult {
	e.t.Helper()
	r := e.run(args...)
	if r.code != 0 {
		e.t.Fatalf("lazycron %s: exit %d\nstdout: %s\nstderr: %s", strings.Join(args, " "), r.code, r.stdout, r.stderr)
	}
	return r
}

// jobs reads the env's crontab back.
func (e *testEnv) jobs() []jobView {
	e.t.Helper()
	var out struct {
		Jobs []jobView `json:"jobs"`
	}
	e.mustRun("ls").decode(e.t, &out)
	return out.Jobs
}

// runCLI executes the real command tree with fresh flag values. stdout is
// not a terminal, so output is JSON unless --json=false is passed.
func runCLI(t *testing.T, args ...string) cliResult {
	t.Helper()
	resetFlags(rootCmd)
	var stdout, stderr bytes.Buffer
	code := executeArgs(args, &stdout, &stderr)
	return cliResult{code: code, stdout: stdout.String(), stderr: stderr.String()}
}

// resetFlags restores every flag in the tree to its default, since cobra
// keeps values in package variables between executions.
func resetFlags(c *cobra.Command) {
	reset := func(f *pflag.Flag) {
		if sv, ok := f.Value.(pflag.SliceValue); ok {
			_ = sv.Replace(nil)
		} else {
			_ = f.Value.Set(f.DefValue)
		}
		f.Changed = false
	}
	c.Flags().VisitAll(reset)
	c.PersistentFlags().VisitAll(reset)
	for _, sub := range c.Commands() {
		resetFlags(sub)
	}
}
