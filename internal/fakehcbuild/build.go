// Package fakehcbuild compiles the fake herdr-core CLI for tests.
package fakehcbuild

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// Build compiles internal/fakehc into a temp dir and points it at a fresh
// state dir. It returns the binary path and the state dir (calls.log lives
// there).
func Build(t testing.TB) (bin, state string) {
	t.Helper()
	dir := t.TempDir()
	bin = filepath.Join(dir, "herdr-core")
	cmd := exec.Command("go", "build", "-o", bin, "github.com/swalha1999/lazycron/internal/fakehc")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake hc: %v\n%s", err, out)
	}
	state = filepath.Join(dir, "state")
	if err := os.MkdirAll(state, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKEHC_STATE", state)
	return bin, state
}
