package cron

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateLabelRejectsUnsafeCharacters(t *testing.T) {
	for _, bad := range []string{"a\n* * * * * evil", "a\rb", "Disk 90% alert", "tab\there", "nul\x00"} {
		if err := ValidateLabel("name", bad); err == nil {
			t.Errorf("%q: expected an error", bad)
		}
	}
	for _, ok := range []string{"", "Backup", "x $(touch /tmp/pwned)", "it's `fine` \"quoted\"", "Ünïcode ✓"} {
		if err := ValidateLabel("name", ok); err != nil {
			t.Errorf("%q: unexpected error %v", ok, err)
		}
	}
}

func TestJobValidateLabelsChecksEveryField(t *testing.T) {
	for _, j := range []Job{
		{Name: "a\nb"},
		{Name: "ok", Tag: "50%"},
		{Name: "ok", TagColor: "#fff\n"},
		{Name: "ok", Project: "p\r"},
	} {
		if err := j.ValidateLabels(); err == nil {
			t.Errorf("%+v: expected an error", j)
		}
	}
}

// Shell metacharacters in a name are single-quoted in the record wrapper
// and round-trip through CrontabLine -> Parse unchanged.
func TestRecordWrapperSingleQuotesName(t *testing.T) {
	name := "x $(touch /tmp/pwned) `id` \"q\" it's $HOME"
	j := Job{ID: "abc", Name: name, Schedule: "0 * * * *", Command: "true", Enabled: true}
	line := j.CrontabLineFor("plain")
	if !strings.Contains(line, ShellQuote(name)) {
		t.Fatalf("name not single-quoted in %q", line)
	}
	jobs := Parse(line)
	if len(jobs) != 1 || jobs[0].Name != name || jobs[0].ID != "abc" {
		t.Fatalf("round trip: %+v", jobs)
	}
}

// Running the wrapper hands the record binary the name verbatim and never
// expands command substitutions in it.
func TestRecordWrapperNeverExpandsName(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	marker := filepath.Join(home, "pwned")
	argsFile := filepath.Join(home, "args")
	writeExecutable(t, RecordBinPath(), "#!/bin/sh\nprintf '%s\\n' \"$2\" > '"+argsFile+"'\n")
	writeExecutable(t, ScriptPath("abc"), "true\n")

	name := "x $(touch " + marker + ") `touch " + marker + "` \"q\""
	cmd := exec.Command("/bin/sh", "-c", WrapWithRecord("bash '"+ScriptPath("abc")+"'", "abc", name))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("wrapper failed: %v\n%s", err, out)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("command substitution in the name was executed")
	}
	got, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSuffix(string(got), "\n") != name {
		t.Fatalf("record got name %q, want %q", got, name)
	}
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}
