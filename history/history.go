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

// Entry represents a single history record.
type Entry struct {
	JobName   string `json:"job_name"`
	Timestamp string `json:"timestamp"`
	Output    string `json:"output"`
	Success   *bool  `json:"success,omitempty"`
	FilePath  string `json:"-"`
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

	var e Entry
	if err := json.Unmarshal(data, &e); err != nil {
		return Entry{}, err
	}
	e.FilePath = path
	return e, nil
}

// WriteEntry writes a history entry to the default history directory.
func WriteEntry(jobName, output string, success bool) error {
	if err := record.EnsureDirs(); err != nil {
		return err
	}
	return WriteEntryTo(record.HistoryDir(), jobName, output, success)
}

// BuildHistoryFile creates the filename and JSON data for a history entry.
func BuildHistoryFile(jobName, output string, success bool) (filename string, data []byte, err error) {
	now := time.Now()
	e := record.Entry{
		JobName:   jobName,
		Timestamp: now.Format(time.RFC3339),
		Output:    output,
		Success:   &success,
	}

	data, err = json.MarshalIndent(e, "", "  ")
	if err != nil {
		return "", nil, err
	}

	safeName := strings.ReplaceAll(jobName, "/", "_")
	safeName = strings.ReplaceAll(safeName, " ", "_")
	filename = now.Format("2006-01-02T15-04-05") + "_" + safeName + ".json"
	return filename, data, nil
}

// WriteEntryTo writes a history entry to the given directory.
func WriteEntryTo(dir, jobName, output string, success bool) error {
	filename, data, err := BuildHistoryFile(jobName, output, success)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, filename), data, 0o600)
}
