package record

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// runRecordScript executes the record shell script with the given input and arguments.
// It sets HOME to a temp directory and returns the parsed entry and output file path.
func runRecordScript(t *testing.T, input string, args ...string) (Entry, string) {
	t.Helper()

	home := t.TempDir()
	scriptPath := filepath.Join(t.TempDir(), "record")

	if err := os.WriteFile(scriptPath, ScriptContent, 0o755); err != nil {
		t.Fatalf("writing script: %v", err)
	}

	cmd := exec.Command("sh", append([]string{scriptPath}, args...)...)
	cmd.Stdin = strings.NewReader(input)
	cmd.Env = []string{"HOME=" + home, "PATH=" + os.Getenv("PATH")}

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("script failed: %v\noutput: %s", err, output)
	}

	histDir := filepath.Join(home, ".lazycron", "history")
	files, err := filepath.Glob(filepath.Join(histDir, "*.json"))
	if err != nil {
		t.Fatalf("globbing: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 JSON file, got %d", len(files))
	}

	data, err := os.ReadFile(files[0])
	if err != nil {
		t.Fatalf("reading JSON: %v", err)
	}

	var entry Entry
	if err := json.Unmarshal(data, &entry); err != nil {
		t.Fatalf("parsing JSON: %v\nraw: %s", err, data)
	}

	return entry, files[0]
}

// --- Basic functionality ---

func TestScript_BasicSuccess(t *testing.T) {
	entry, _ := runRecordScript(t, "hello world", "abc12345", "test-job", "0")

	if entry.JobID != "abc12345" {
		t.Errorf("JobID = %q, want %q", entry.JobID, "abc12345")
	}
	if entry.JobName != "test-job" {
		t.Errorf("JobName = %q, want %q", entry.JobName, "test-job")
	}
	if entry.Output != "hello world" {
		t.Errorf("Output = %q, want %q", entry.Output, "hello world")
	}
	if entry.Success == nil || *entry.Success != true {
		t.Errorf("Success = %v, want true", entry.Success)
	}
}

func TestScript_NonZeroExit(t *testing.T) {
	entry, _ := runRecordScript(t, "error output", "def67890", "fail-job", "1")

	if entry.JobName != "fail-job" {
		t.Errorf("JobName = %q, want %q", entry.JobName, "fail-job")
	}
	if entry.Output != "error output" {
		t.Errorf("Output = %q, want %q", entry.Output, "error output")
	}
	if entry.Success == nil || *entry.Success != false {
		t.Errorf("Success = %v, want false", entry.Success)
	}
}

func TestScript_ExitCode127(t *testing.T) {
	entry, _ := runRecordScript(t, "command not found", "aabb1122", "missing-cmd", "127")

	if entry.Success == nil || *entry.Success != false {
		t.Errorf("Success = %v, want false (exit 127)", entry.Success)
	}
}

func TestScript_DefaultExitCode(t *testing.T) {
	entry, _ := runRecordScript(t, "output", "ccdd3344", "default-exit")

	if entry.Success == nil || *entry.Success != true {
		t.Errorf("Success = %v, want true (default exit 0)", entry.Success)
	}
}

func TestScript_EmptyOutput(t *testing.T) {
	entry, _ := runRecordScript(t, "", "eeff5566", "empty-job", "0")

	if entry.Output != "" {
		t.Errorf("Output = %q, want empty", entry.Output)
	}
}

// --- Multi-line output ---

func TestScript_MultiLineOutput(t *testing.T) {
	input := "line1\nline2\nline3"
	entry, _ := runRecordScript(t, input, "a1000001", "multi-line", "0")

	if entry.Output != input {
		t.Errorf("Output = %q, want %q", entry.Output, input)
	}
}

func TestScript_TrailingNewline(t *testing.T) {
	// Command substitution strips trailing newlines, so "hello\n" becomes "hello"
	entry, _ := runRecordScript(t, "hello\n", "a1000002", "trail-nl", "0")

	if entry.Output != "hello" {
		t.Errorf("Output = %q, want %q", entry.Output, "hello")
	}
}

// --- Special character escaping ---

func TestScript_OutputWithQuotes(t *testing.T) {
	input := `he said "hello" and "goodbye"`
	entry, _ := runRecordScript(t, input, "a1000003", "quotes-job", "0")

	if entry.Output != input {
		t.Errorf("Output = %q, want %q", entry.Output, input)
	}
}

func TestScript_OutputWithBackslashes(t *testing.T) {
	input := `path\to\file`
	entry, _ := runRecordScript(t, input, "a1000004", "backslash-job", "0")

	if entry.Output != input {
		t.Errorf("Output = %q, want %q", entry.Output, input)
	}
}

func TestScript_OutputWithTabs(t *testing.T) {
	input := "col1\tcol2\tcol3"
	entry, _ := runRecordScript(t, input, "a1000005", "tab-job", "0")

	if entry.Output != input {
		t.Errorf("Output = %q, want %q", entry.Output, input)
	}
}

func TestScript_OutputWithDollarSign(t *testing.T) {
	input := "price is $100 and $HOME is set"
	entry, _ := runRecordScript(t, input, "a1000006", "dollar-job", "0")

	if entry.Output != input {
		t.Errorf("Output = %q, want %q", entry.Output, input)
	}
}

func TestScript_OutputWithBackticks(t *testing.T) {
	input := "result is `command` output"
	entry, _ := runRecordScript(t, input, "a1000007", "backtick-job", "0")

	if entry.Output != input {
		t.Errorf("Output = %q, want %q", entry.Output, input)
	}
}

func TestScript_OutputWithSingleQuotes(t *testing.T) {
	input := "it's a test with 'quotes'"
	entry, _ := runRecordScript(t, input, "a1000008", "single-quote-job", "0")

	if entry.Output != input {
		t.Errorf("Output = %q, want %q", entry.Output, input)
	}
}

func TestScript_OutputWithBraces(t *testing.T) {
	input := `{"key": "value", "arr": [1, 2, 3]}`
	entry, _ := runRecordScript(t, input, "a1000009", "braces-job", "0")

	if entry.Output != input {
		t.Errorf("Output = %q, want %q", entry.Output, input)
	}
}

func TestScript_OutputWithSymbols(t *testing.T) {
	input := "!@#$%^&*()_+-=[]{}|;':,./<>?"
	entry, _ := runRecordScript(t, input, "a100000a", "symbols-job", "0")

	if entry.Output != input {
		t.Errorf("Output = %q, want %q", entry.Output, input)
	}
}

func TestScript_OutputWithMixedSpecialChars(t *testing.T) {
	input := "line1: \"quoted\"\nline2: back\\slash\nline3: tab\there\nline4: $var and `cmd`"
	entry, _ := runRecordScript(t, input, "a100000b", "mixed-job", "0")

	if entry.Output != input {
		t.Errorf("Output = %q, want %q", entry.Output, input)
	}
}

func TestScript_OutputWithUnicode(t *testing.T) {
	input := "unicode: caf\u00e9 r\u00e9sum\u00e9 \u2603 \u2764"
	entry, _ := runRecordScript(t, input, "a100000c", "unicode-job", "0")

	if entry.Output != input {
		t.Errorf("Output = %q, want %q", entry.Output, input)
	}
}

// --- Job name sanitization ---

func TestScript_JobNameWithSlash(t *testing.T) {
	entry, path := runRecordScript(t, "output", "a100000d", "path/to/job", "0")

	if entry.JobName != "path/to/job" {
		t.Errorf("JobName = %q, want %q", entry.JobName, "path/to/job")
	}

	filename := filepath.Base(path)
	// Filename should use ID, not sanitized name
	if !strings.Contains(filename, "a100000d") {
		t.Errorf("filename should contain job ID: %s", filename)
	}
}

func TestScript_JobNameWithSpaces(t *testing.T) {
	entry, path := runRecordScript(t, "output", "a100000e", "my job name", "0")

	if entry.JobName != "my job name" {
		t.Errorf("JobName = %q, want %q", entry.JobName, "my job name")
	}

	filename := filepath.Base(path)
	// Filename should use ID, not sanitized name
	if !strings.Contains(filename, "a100000e") {
		t.Errorf("filename should contain job ID: %s", filename)
	}
}

// --- File and directory creation ---

func TestScript_CreatesHistoryDir(t *testing.T) {
	home := t.TempDir()
	histDir := filepath.Join(home, ".lazycron", "history")

	if _, err := os.Stat(histDir); !os.IsNotExist(err) {
		t.Fatal("history dir should not exist yet")
	}

	scriptPath := filepath.Join(t.TempDir(), "record")
	os.WriteFile(scriptPath, ScriptContent, 0o755)

	cmd := exec.Command("sh", scriptPath, "a100000f", "dir-test", "0")
	cmd.Stdin = strings.NewReader("hello")
	cmd.Env = []string{"HOME=" + home, "PATH=" + os.Getenv("PATH")}

	if err := cmd.Run(); err != nil {
		t.Fatalf("script failed: %v", err)
	}

	if _, err := os.Stat(histDir); os.IsNotExist(err) {
		t.Error("history dir was not created")
	}
}

func TestScript_HistoryDirAlreadyExists(t *testing.T) {
	home := t.TempDir()
	histDir := filepath.Join(home, ".lazycron", "history")
	os.MkdirAll(histDir, 0o755)

	scriptPath := filepath.Join(t.TempDir(), "record")
	os.WriteFile(scriptPath, ScriptContent, 0o755)

	cmd := exec.Command("sh", scriptPath, "a1000010", "dir-test", "0")
	cmd.Stdin = strings.NewReader("hello")
	cmd.Env = []string{"HOME=" + home, "PATH=" + os.Getenv("PATH")}

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("script should not fail when history dir exists: %v\n%s", err, output)
	}

	files, _ := filepath.Glob(filepath.Join(histDir, "*.json"))
	if len(files) != 1 {
		t.Fatalf("expected 1 JSON file, got %d", len(files))
	}
}

func TestScript_FilenameFormat(t *testing.T) {
	_, path := runRecordScript(t, "hello", "a1000011", "format-job", "0")

	filename := filepath.Base(path)
	if !strings.HasSuffix(filename, "_a1000011.json") {
		t.Errorf("unexpected filename suffix: %s, want _a1000011.json", filename)
	}
	// Should start with YYYY-MM-DDTHH-MM-SS
	if len(filename) < 20 {
		t.Errorf("filename too short: %s", filename)
	}
	if filename[4] != '-' || filename[7] != '-' || filename[10] != 'T' {
		t.Errorf("filename doesn't match date format: %s", filename)
	}
}

// --- Timestamp ---

func TestScript_TimestampPresent(t *testing.T) {
	entry, _ := runRecordScript(t, "hello", "a1000012", "ts-job", "0")

	if entry.Timestamp == "" {
		t.Fatal("Timestamp is empty")
	}
	if len(entry.Timestamp) < 19 {
		t.Errorf("Timestamp too short: %q", entry.Timestamp)
	}
	// Should start with YYYY-MM-DD
	if entry.Timestamp[4] != '-' || entry.Timestamp[7] != '-' || entry.Timestamp[10] != 'T' {
		t.Errorf("Timestamp doesn't match ISO format: %q", entry.Timestamp)
	}
}

// --- JSON validity ---

func TestScript_ValidJSON(t *testing.T) {
	inputs := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"simple", "simple text"},
		{"quotes", `with "quotes"`},
		{"newlines", "with\nnewlines"},
		{"tabs", "with\ttabs"},
		{"backslashes", `with \backslashes\`},
		{"mixed escapes", "mixed: \"\t\n\\"},
		{"unicode", "caf\u00e9 r\u00e9sum\u00e9"},
		{"symbols", "!@#$%^&*()"},
		{"json-like", `{"key": [1,2,3]}`},
		{"dollar signs", "$HOME $PATH $100"},
		{"backticks", "`echo hi` `date`"},
		{"single quotes", "it's a 'test'"},
		{"angle brackets", "<html>&amp;</html>"},
		{"percent signs", "100% done %s %d"},
	}

	for _, tt := range inputs {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			scriptPath := filepath.Join(t.TempDir(), "record")
			os.WriteFile(scriptPath, ScriptContent, 0o755)

			cmd := exec.Command("sh", scriptPath, "a1000013", "json-test", "0")
			cmd.Stdin = strings.NewReader(tt.input)
			cmd.Env = []string{"HOME=" + home, "PATH=" + os.Getenv("PATH")}

			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("script failed: %v\n%s", err, output)
			}

			files, _ := filepath.Glob(filepath.Join(home, ".lazycron", "history", "*.json"))
			if len(files) != 1 {
				t.Fatalf("expected 1 file, got %d", len(files))
			}

			data, _ := os.ReadFile(files[0])
			var entry map[string]interface{}
			if err := json.Unmarshal(data, &entry); err != nil {
				t.Errorf("invalid JSON for input %q: %v\nraw: %s", tt.input, err, data)
			}
		})
	}
}

// --- Error handling ---

func TestScript_NoArgs(t *testing.T) {
	scriptPath := filepath.Join(t.TempDir(), "record")
	os.WriteFile(scriptPath, ScriptContent, 0o755)

	// No args at all
	cmd := exec.Command("sh", scriptPath)
	cmd.Stdin = strings.NewReader("hello")
	cmd.Env = []string{"HOME=" + t.TempDir(), "PATH=" + os.Getenv("PATH")}

	err := cmd.Run()
	if err == nil {
		t.Error("expected script to fail with no args")
	}

	// Only one arg (needs at least 2: id + name)
	cmd2 := exec.Command("sh", scriptPath, "abc12345")
	cmd2.Stdin = strings.NewReader("hello")
	cmd2.Env = []string{"HOME=" + t.TempDir(), "PATH=" + os.Getenv("PATH")}

	err = cmd2.Run()
	if err == nil {
		t.Error("expected script to fail with only 1 arg")
	}
}

// --- Compatibility with Go history loader ---

func TestScript_CompatibleWithGoEntry(t *testing.T) {
	entry, _ := runRecordScript(t, "test output", "a1000014", "compat-job", "0")

	// Verify all fields expected by history.LoadEntry / record.Entry
	if entry.JobName == "" {
		t.Error("JobName is empty")
	}
	if entry.Timestamp == "" {
		t.Error("Timestamp is empty")
	}
	if entry.Success == nil {
		t.Error("Success is nil")
	}
}

// --- Multiple runs ---

func TestScript_TwoRunsProduceTwoFiles(t *testing.T) {
	home := t.TempDir()
	scriptPath := filepath.Join(t.TempDir(), "record")
	os.WriteFile(scriptPath, ScriptContent, 0o755)

	env := []string{"HOME=" + home, "PATH=" + os.Getenv("PATH")}

	cmd1 := exec.Command("sh", scriptPath, "a1000015", "myjob", "0")
	cmd1.Stdin = strings.NewReader("first")
	cmd1.Env = env
	if err := cmd1.Run(); err != nil {
		t.Fatalf("first run failed: %v", err)
	}

	// Sleep 1 second so the timestamp-based filename differs
	time.Sleep(1 * time.Second)

	cmd2 := exec.Command("sh", scriptPath, "a1000016", "myjob", "0")
	cmd2.Stdin = strings.NewReader("second")
	cmd2.Env = env
	if err := cmd2.Run(); err != nil {
		t.Fatalf("second run failed: %v", err)
	}

	histDir := filepath.Join(home, ".lazycron", "history")
	files, _ := filepath.Glob(filepath.Join(histDir, "*.json"))
	if len(files) != 2 {
		t.Errorf("expected 2 JSON files, got %d", len(files))
	}
}

// --- InstallRecord ---

func TestInstallRecord_WritesScript(t *testing.T) {
	// Override HOME to a temp dir so we don't write to real ~/.lazycron
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := InstallRecord(); err != nil {
		t.Fatalf("InstallRecord() error: %v", err)
	}

	path := filepath.Join(home, ".lazycron", "bin", "record")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading installed script: %v", err)
	}

	if !strings.HasPrefix(string(data), "#!/bin/sh") {
		t.Error("installed file should start with #!/bin/sh shebang")
	}

	info, _ := os.Stat(path)
	if info.Mode().Perm()&0o111 == 0 {
		t.Error("installed script should be executable")
	}
}

func TestInstallRecord_Idempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := InstallRecord(); err != nil {
		t.Fatalf("first install: %v", err)
	}
	if err := InstallRecord(); err != nil {
		t.Fatalf("second install: %v", err)
	}

	path := filepath.Join(home, ".lazycron", "bin", "record")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading installed script: %v", err)
	}
	if string(data) != string(ScriptContent) {
		t.Error("installed content should match embedded script")
	}
}
