package record

import (
	"os"
	"path/filepath"
	"strings"
)

// SelfLinkPath returns ~/.lazycron/bin/lazycron, the stable path crontab
// lines use to call lazycron.
func SelfLinkPath() string {
	return filepath.Join(BinDir(), "lazycron")
}

// executable is os.Executable, swappable in tests.
var executable = os.Executable

// InstallSelfLink points ~/.lazycron/bin/lazycron at the running binary so
// crontab lines keep working wherever lazycron was installed. It is best
// effort and skips throwaway binaries (go run, go test).
func InstallSelfLink() {
	exe, err := executable()
	if err != nil || isThrowawayBinary(exe) {
		return
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	link := SelfLinkPath()
	if current, err := os.Readlink(link); err == nil && current == exe {
		return
	}
	if err := EnsureDirs(); err != nil {
		return
	}
	tmp := link + ".new"
	_ = os.Remove(tmp)
	if err := os.Symlink(exe, tmp); err != nil {
		return
	}
	_ = os.Rename(tmp, link)
}

// isThrowawayBinary reports whether exe lives in a temp build directory.
func isThrowawayBinary(exe string) bool {
	tmp := os.TempDir()
	return strings.HasPrefix(exe, tmp) || strings.Contains(exe, "go-build") || strings.HasSuffix(exe, ".test")
}
