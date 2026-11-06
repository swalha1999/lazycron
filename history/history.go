package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/swalha1999/lazycron/record"
)

// Entry represents a single history record: one finished run of a job.
// Entries written by the shell `record` script only have the first five
// fields; runs started by `lazycron exec` fill in the rest.
type Entry struct {
	JobID     string `json:"job_id,omitempty"`
	JobName   string `json:"job_name"`
	Timestamp string `json:"timestamp"` // when the run finished (RFC3339)
	Output    string `json:"output"`
	Success   *bool  `json:"success,omitempty"`

	RunID           string `json:"run_id,omitempty"`
	StartedAt       string `json:"started_at,omitempty"` // RFC3339
	ExitCode        *int   `json:"exit_code,omitempty"`
	Signal          int    `json:"signal,omitempty"`  // set when the run was killed
	Runner          string `json:"runner,omitempty"`  // "plain" or "herdr"
	Trigger         string `json:"trigger,omitempty"` // "cron" or "manual"
	Terminal        string `json:"terminal,omitempty"`
	TerminalName    string `json:"terminal_name,omitempty"`
	OutputTruncated bool   `json:"output_truncated,omitempty"`
	FallbackReason  string `json:"fallback_reason,omitempty"`

	FilePath string `json:"-"`
}

// IDFromPath derives a run id from a history file path: its base name
// without ".json". Every entry gets a stable id this way, including ones
// written before run ids existed.
func IDFromPath(path string) string {
	return strings.TrimSuffix(filepath.Base(path), ".json")
}

// LoadAll reads all JSON files from ~/.lazycron/history/ sorted by timestamp desc.
func LoadAll() ([]Entry, error) {
	return LoadAllFrom(record.HistoryDir())
}

// LoadAllFrom reads all JSON files from the given directory sorted by timestamp desc.
func LoadAllFrom(dir string) ([]Entry, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}

	var entries []Entry
	for _, f := range files {
		e, err := LoadEntry(f)
		if err != nil {
			continue
		}
		entries = append(entries, e)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp > entries[j].Timestamp
	})

	return entries, nil
}

// LoadEntry reads a single history JSON file.
func LoadEntry(path string) (Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Entry{}, err
	}

	return ParseEntry(path, data)
}

// ParseEntry decodes a history file's contents, filling FilePath and a
// derived RunID when the file has none.
func ParseEntry(path string, data []byte) (Entry, error) {
	var e Entry
	if err := json.Unmarshal(data, &e); err != nil {
		return Entry{}, err
	}
	e.FilePath = path
	if e.RunID == "" {
		e.RunID = IDFromPath(path)
	}
	return e, nil
}

// EntryFileName returns the history file name for a run.
func EntryFileName(runID string) string {
	return runID + ".json"
}

// MarshalEntry encodes an entry the way history files are stored.
func MarshalEntry(e Entry) ([]byte, error) {
	return json.MarshalIndent(e, "", "  ")
}

// WriteRunEntry writes a run's entry to dir as <run-id>.json.
func WriteRunEntry(dir string, e Entry) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := MarshalEntry(e)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, EntryFileName(e.RunID)), data, 0o600)
}

// WriteEntry writes a history entry to the default history directory.
func WriteEntry(jobID, jobName, output string, success bool) error {
	if err := record.EnsureDirs(); err != nil {
		return err
	}
	return WriteEntryTo(record.HistoryDir(), jobID, jobName, output, success)
}

// BuildHistoryFile creates the filename and JSON data for a history entry.
func BuildHistoryFile(jobID, jobName, output string, success bool) (filename string, data []byte, err error) {
	now := time.Now()
	e := Entry{
		JobID:     jobID,
		JobName:   jobName,
		Timestamp: now.Format(time.RFC3339),
		Output:    output,
		Success:   &success,
	}

	data, err = MarshalEntry(e)
	if err != nil {
		return "", nil, err
	}

	filename = now.Format("2006-01-02T15-04-05") + "_" + jobID + ".json"
	return filename, data, nil
}

// WriteEntryTo writes a history entry to the given directory.
func WriteEntryTo(dir, jobID, jobName, output string, success bool) error {
	filename, data, err := BuildHistoryFile(jobID, jobName, output, success)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, filename), data, 0o600)
}
