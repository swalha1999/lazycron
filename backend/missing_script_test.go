package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/swalha1999/lazycron/cron"
)

// Regression for #130 on the file backend: missing script -> read -> write
// must not leave a script that runs itself.
func TestFileBackend_MissingScriptNotSelfReferenced(t *testing.T) {
	dir := t.TempDir()
	fb := NewFileBackend(filepath.Join(dir, "crontab"), filepath.Join(dir, "history"))
	jobs := []cron.Job{
		{ID: "abc12345", Name: "a", Schedule: "* * * * *", Command: "echo a", Enabled: true},
		{ID: "def67890", Name: "b", Schedule: "* * * * *", Command: "echo b", Enabled: true},
	}
	if err := fb.WriteJobs(jobs); err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(fb.ScriptsDir(), "abc12345.sh")
	if err := os.Remove(missing); err != nil {
		t.Fatal(err)
	}

	got, err := fb.ReadJobs()
	if err != nil {
		t.Fatal(err)
	}
	if !got[0].ScriptMissing() || got[1].ScriptMissing() {
		t.Fatalf("ScriptMissing = %v, %v; want true, false", got[0].ScriptMissing(), got[1].ScriptMissing())
	}
	got[1].Command = "echo edited"
	if err := fb.WriteJobs(got); err != nil {
		t.Fatalf("WriteJobs: %v", err)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Errorf("missing script was rewritten (err=%v)", err)
	}
	if cmd, _ := cron.ReadScriptCommand(filepath.Join(fb.ScriptsDir(), "def67890.sh")); cmd != "echo edited" {
		t.Errorf("edited job's script = %q", cmd)
	}
}

type fakeUploader map[string]string

func (f fakeUploader) Upload(content, path string, _ os.FileMode) error {
	f[path] = content
	return nil
}

// Regression for #130 on the remote backend: a script-ref command is never
// uploaded as its own script, but its file still counts as active so the
// orphan sweep does not delete it.
func TestUploadScripts_SkipsMissingScript(t *testing.T) {
	dir := "/home/u/.lazycron/scripts"
	jobs := []cron.Job{
		{ID: "abc12345", Name: "a", Command: "bash '" + dir + "/abc12345.sh'"},
		{ID: "def67890", Name: "b", Command: "echo b"},
	}
	up := fakeUploader{}
	active, err := uploadScripts(up, dir, jobs)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := up[dir+"/abc12345.sh"]; ok {
		t.Error("uploaded a script that runs itself")
	}
	if !strings.HasSuffix(up[dir+"/def67890.sh"], "echo b\n") {
		t.Errorf("job b script = %q", up[dir+"/def67890.sh"])
	}
	if !active["abc12345.sh"] || !active["def67890.sh"] {
		t.Errorf("active = %v, want both scripts", active)
	}
}
