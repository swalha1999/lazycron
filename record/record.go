package record

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Entry is the JSON structure written to history files.
type Entry struct {
	JobName   string `json:"job_name"`
	Timestamp string `json:"timestamp"`
	Output    string `json:"output"`
	Success   *bool  `json:"success,omitempty"`
}

// Run is the entrypoint when the binary is invoked as "record".
// Usage: <command> | record "job-name" [exit-code]
func Run(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "usage: record <job-name> [exit-code]")
		os.Exit(1)
	}

	jobName := args[0]
	now := time.Now()

	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "record: reading stdin: %v\n", err)
		os.Exit(1)
	}

	entry := Entry{
		JobName:   jobName,
		Timestamp: now.Format(time.RFC3339),
		Output:    string(data),
	}

	// Parse optional exit code argument
	if len(args) >= 2 {
		success := args[1] == "0"
		entry.Success = &success
	}

	histDir := HistoryDir()
	if err := os.MkdirAll(histDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "record: creating history dir: %v\n", err)
		os.Exit(1)
	}

	// Sanitize job name for filename
	safeName := strings.ReplaceAll(jobName, "/", "_")
	safeName = strings.ReplaceAll(safeName, " ", "_")

	filename := fmt.Sprintf("%s_%s.json", now.Format("2006-01-02T15-04-05"), safeName)
	path := filepath.Join(histDir, filename)

	jsonData, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "record: marshaling json: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(path, jsonData, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "record: writing file: %v\n", err)
		os.Exit(1)
	}
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

// RecordPath returns the full path to the record binary.
func RecordPath() string {
	return filepath.Join(BinDir(), "record")
}

// EnsureDirs creates ~/.lazycron/bin/ and ~/.lazycron/history/.
func EnsureDirs() error {
	if err := os.MkdirAll(BinDir(), 0o755); err != nil {
		return err
	}
	return os.MkdirAll(HistoryDir(), 0o755)
}

// InstallRecord copies the current binary to ~/.lazycron/bin/record.
func InstallRecord() error {
	if err := EnsureDirs(); err != nil {
		return err
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("finding executable: %w", err)
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return fmt.Errorf("resolving symlinks: %w", err)
	}

	src, err := os.ReadFile(exePath)
	if err != nil {
		return fmt.Errorf("reading executable: %w", err)
	}

	dst := RecordPath()
	if err := os.WriteFile(dst, src, 0o755); err != nil {
		return fmt.Errorf("writing record binary: %w", err)
	}

	return nil
}
