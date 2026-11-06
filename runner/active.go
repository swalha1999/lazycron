package runner

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

// Triggers say what started a run.
const (
	TriggerCron   = "cron"
	TriggerManual = "manual"
)

// Active describes a run in progress. It is stored as Paths.Runs/<run-id>.json
// from start until the history entry is written.
type Active struct {
	RunID        string `json:"run_id"`
	JobID        string `json:"job_id"`
	JobName      string `json:"job_name"`
	Runner       string `json:"runner"`
	Trigger      string `json:"trigger"`
	StartedAt    string `json:"started_at"`
	Terminal     string `json:"terminal,omitempty"`      // herdr address, e.g. t17
	TerminalName string `json:"terminal_name,omitempty"` // herdr name
	PID          int    `json:"pid,omitempty"`           // plain runs: the job's process (group leader)
	ExecPID      int    `json:"exec_pid"`                // the lazycron process supervising the run
	Stale        bool   `json:"stale,omitempty"`         // supervisor is gone; the run was lost
}

// timeLayout is the timestamp prefix of run ids, sortable as text.
const timeLayout = "2006-01-02T15-04-05"

// NewRunID returns a unique, sortable run id: <start time>_<job id>_<4 hex>.
func NewRunID(jobID string, start time.Time) string {
	b := make([]byte, 2)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return start.Format(timeLayout) + "_" + jobID + "_" + hex.EncodeToString(b)
}

// shortSuffix returns the random tail of a run id (for terminal names).
func shortSuffix(runID string) string {
	if i := strings.LastIndexByte(runID, '_'); i >= 0 {
		return runID[i+1:]
	}
	return runID
}

func markerPath(dir, runID string) string {
	return filepath.Join(dir, runID+".json")
}

// WriteActive stores (or updates) a run's marker.
func WriteActive(dir string, a Active) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	tmp := markerPath(dir, a.RunID) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, markerPath(dir, a.RunID))
}

// RemoveActive deletes a run's marker and exit file.
func RemoveActive(dir, runID string) {
	_ = os.Remove(markerPath(dir, runID))
	_ = os.Remove(filepath.Join(dir, runID+".exit"))
}

// ParseActive decodes a marker file's contents.
func ParseActive(data []byte) (Active, error) {
	var a Active
	if err := json.Unmarshal(data, &a); err != nil {
		return Active{}, err
	}
	if a.RunID == "" {
		return Active{}, errors.New("marker without run_id")
	}
	return a, nil
}

// LoadActive returns the runs in progress in dir, newest first. Markers
// whose supervising process no longer exists are returned with Stale set.
func LoadActive(dir string) ([]Active, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	var runs []Active
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		a, err := ParseActive(data)
		if err != nil {
			continue
		}
		a.Stale = a.ExecPID > 0 && !processAlive(a.ExecPID)
		runs = append(runs, a)
	}
	SortActive(runs)
	return runs, nil
}

// SortActive orders runs newest first.
func SortActive(runs []Active) {
	sort.Slice(runs, func(i, j int) bool { return runs[i].RunID > runs[j].RunID })
}

// processAlive reports whether a local process exists.
func processAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// FindActive returns the active run with the given id.
func FindActive(runs []Active, runID string) (Active, bool) {
	for _, a := range runs {
		if a.RunID == runID {
			return a, true
		}
	}
	return Active{}, false
}

// readExitFile returns the exit code a herdr wrapper wrote, if any.
func readExitFile(path string) (int, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	var code int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(data)), "%d", &code); err != nil {
		return 0, false
	}
	return code, true
}
