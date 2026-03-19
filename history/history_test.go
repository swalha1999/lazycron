package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// --- LoadEntry ---

func TestLoadEntry(t *testing.T) {
	dir := t.TempDir()
	data := `{"job_name":"test-job","timestamp":"2025-06-15T10:30:00Z","output":"hello","success":true}`
	path := filepath.Join(dir, "entry.json")
	os.WriteFile(path, []byte(data), 0o644)

	entry, err := LoadEntry(path)
	if err != nil {
		t.Fatalf("LoadEntry error: %v", err)
	}

	if entry.JobName != "test-job" {
		t.Errorf("JobName = %q, want %q", entry.JobName, "test-job")
	}
	if entry.Timestamp != "2025-06-15T10:30:00Z" {
		t.Errorf("Timestamp = %q, want %q", entry.Timestamp, "2025-06-15T10:30:00Z")
	}
	if entry.Output != "hello" {
		t.Errorf("Output = %q, want %q", entry.Output, "hello")
	}
	if entry.Success == nil || *entry.Success != true {
		t.Errorf("Success = %v, want true", entry.Success)
	}
	if entry.FilePath != path {
		t.Errorf("FilePath = %q, want %q", entry.FilePath, path)
	}
}

func TestLoadEntry_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	os.WriteFile(path, []byte("not json"), 0o644)

	_, err := LoadEntry(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadEntry_NonexistentFile(t *testing.T) {
	_, err := LoadEntry("/nonexistent/path/file.json")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoadEntry_NullSuccess(t *testing.T) {
	dir := t.TempDir()
	data := `{"job_name":"test","timestamp":"2025-01-01T00:00:00Z","output":""}`
	path := filepath.Join(dir, "entry.json")
	os.WriteFile(path, []byte(data), 0o644)

	entry, err := LoadEntry(path)
	if err != nil {
		t.Fatalf("LoadEntry error: %v", err)
	}
	if entry.Success != nil {
		t.Errorf("Success should be nil when omitted, got %v", *entry.Success)
	}
}

// --- LoadAll ---

func TestLoadAll_SortsDescending(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	histDir := filepath.Join(home, ".lazycron", "history")
	os.MkdirAll(histDir, 0o755)

	entries := []struct {
		filename  string
		timestamp string
	}{
		{"2025-06-15T10-00-00_job-a.json", "2025-06-15T10:00:00Z"},
		{"2025-06-15T12-00-00_job-b.json", "2025-06-15T12:00:00Z"},
		{"2025-06-15T08-00-00_job-c.json", "2025-06-15T08:00:00Z"},
	}

	for _, e := range entries {
		data, _ := json.Marshal(map[string]interface{}{
			"job_name":  "test",
			"timestamp": e.timestamp,
			"output":    "",
			"success":   true,
		})
		os.WriteFile(filepath.Join(histDir, e.filename), data, 0o644)
	}

	result, err := LoadAll()
	if err != nil {
		t.Fatalf("LoadAll error: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(result))
	}

	// Should be sorted descending by timestamp
	if result[0].Timestamp != "2025-06-15T12:00:00Z" {
		t.Errorf("first entry timestamp = %q, want newest", result[0].Timestamp)
	}
	if result[2].Timestamp != "2025-06-15T08:00:00Z" {
		t.Errorf("last entry timestamp = %q, want oldest", result[2].Timestamp)
	}
}

func TestLoadAll_EmptyDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	histDir := filepath.Join(home, ".lazycron", "history")
	os.MkdirAll(histDir, 0o755)

	result, err := LoadAll()
	if err != nil {
		t.Fatalf("LoadAll error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 entries, got %d", len(result))
	}
}

func TestLoadAll_SkipsInvalidFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	histDir := filepath.Join(home, ".lazycron", "history")
	os.MkdirAll(histDir, 0o755)

	// Valid entry
	validData, _ := json.Marshal(map[string]interface{}{
		"job_name":  "valid",
		"timestamp": "2025-06-15T10:00:00Z",
		"output":    "ok",
		"success":   true,
	})
	os.WriteFile(filepath.Join(histDir, "valid.json"), validData, 0o644)

	// Invalid entry
	os.WriteFile(filepath.Join(histDir, "invalid.json"), []byte("not json"), 0o644)

	result, err := LoadAll()
	if err != nil {
		t.Fatalf("LoadAll error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 valid entry, got %d", len(result))
	}
	if result[0].JobName != "valid" {
		t.Errorf("JobName = %q, want %q", result[0].JobName, "valid")
	}
}

// --- WriteEntry ---

func TestWriteEntry(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	err := WriteEntry("my-job", "hello output", true)
	if err != nil {
		t.Fatalf("WriteEntry error: %v", err)
	}

	histDir := filepath.Join(home, ".lazycron", "history")
	files, err := filepath.Glob(filepath.Join(histDir, "*.json"))
	if err != nil {
		t.Fatalf("glob error: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	entry, err := LoadEntry(files[0])
	if err != nil {
		t.Fatalf("LoadEntry error: %v", err)
	}

	if entry.JobName != "my-job" {
		t.Errorf("JobName = %q, want %q", entry.JobName, "my-job")
	}
	if entry.Output != "hello output" {
		t.Errorf("Output = %q, want %q", entry.Output, "hello output")
	}
	if entry.Success == nil || *entry.Success != true {
		t.Errorf("Success = %v, want true", entry.Success)
	}
	if entry.Timestamp == "" {
		t.Error("Timestamp is empty")
	}
}

func TestWriteEntry_Failure(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	err := WriteEntry("fail-job", "error msg", false)
	if err != nil {
		t.Fatalf("WriteEntry error: %v", err)
	}

	histDir := filepath.Join(home, ".lazycron", "history")
	files, _ := filepath.Glob(filepath.Join(histDir, "*.json"))
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	entry, _ := LoadEntry(files[0])
	if entry.Success == nil || *entry.Success != false {
		t.Errorf("Success = %v, want false", entry.Success)
	}
}

func TestWriteEntry_SanitizesJobName(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	err := WriteEntry("path/to/job name", "output", true)
	if err != nil {
		t.Fatalf("WriteEntry error: %v", err)
	}

	histDir := filepath.Join(home, ".lazycron", "history")
	files, _ := filepath.Glob(filepath.Join(histDir, "*.json"))
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	filename := filepath.Base(files[0])
	// Slashes and spaces should be replaced with underscores
	if filepath.Dir(files[0]) != histDir {
		t.Error("file created in wrong directory")
	}
	// The filename should not contain slashes or spaces
	for _, ch := range filename {
		if ch == '/' || ch == ' ' {
			t.Errorf("filename %q contains invalid character %q", filename, string(ch))
		}
	}
}

func TestWriteEntry_CreatesDirs(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	histDir := filepath.Join(home, ".lazycron", "history")
	if _, err := os.Stat(histDir); !os.IsNotExist(err) {
		t.Fatal("history dir should not exist yet")
	}

	err := WriteEntry("test", "output", true)
	if err != nil {
		t.Fatalf("WriteEntry error: %v", err)
	}

	if _, err := os.Stat(histDir); os.IsNotExist(err) {
		t.Error("WriteEntry should create history dir")
	}
}
