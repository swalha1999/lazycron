package record

import (
	_ "embed"
	"os"
	"path/filepath"
)

//go:embed record.sh
var ScriptContent []byte

// HistoryDir returns ~/.lazycron/history/
func HistoryDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".lazycron", "history")
}

// BinDir returns ~/.lazycron/bin/
func BinDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".lazycron", "bin")
}

// RecordPath returns the full path to the record script.
func RecordPath() string {
	return filepath.Join(BinDir(), "record")
}

// EnsureDirs creates ~/.lazycron/bin/ and ~/.lazycron/history/.
func EnsureDirs() error {
	if err := os.MkdirAll(BinDir(), 0o700); err != nil {
		return err
	}
	return os.MkdirAll(HistoryDir(), 0o700)
}

// InstallRecord writes the embedded POSIX shell script to ~/.lazycron/bin/record.
func InstallRecord() error {
	if err := EnsureDirs(); err != nil {
		return err
	}
	return os.WriteFile(RecordPath(), ScriptContent, 0o755)
}
