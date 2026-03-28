package record

import (
	_ "embed"
	"os"
	"path/filepath"
)

//go:embed record.sh
var ScriptContent []byte

//go:embed notify.sh
var notifyScript []byte

// Entry is the JSON structure written to history files.
type Entry struct {
	JobID     string `json:"job_id,omitempty"`
	JobName   string `json:"job_name"`
	Timestamp string `json:"timestamp"`
	Output    string `json:"output"`
	Success   *bool  `json:"success,omitempty"`
}

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

// NotifyPath returns the full path to the notify script.
func NotifyPath() string {
	return filepath.Join(BinDir(), "notify")
}

// InstallRecord writes the embedded POSIX shell scripts to ~/.lazycron/bin/.
func InstallRecord() error {
	if err := EnsureDirs(); err != nil {
		return err
	}
	if err := os.WriteFile(RecordPath(), ScriptContent, 0o755); err != nil {
		return err
	}
	return os.WriteFile(NotifyPath(), notifyScript, 0o755)
}
