package backend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/swalha1999/lazycron/cron"
)

const handWritten = "MAILTO=me@example.com\n@reboot /home/u/start.sh\n@daily /usr/bin/find /tmp -mtime +7 -delete\n"

var newJob = cron.Job{ID: "abc12345", Name: "hello", Schedule: "* * * * *", Command: "echo hi", Enabled: true}

func assertKeepsHandWritten(t *testing.T, crontab string) {
	t.Helper()
	if !strings.HasPrefix(crontab, handWritten) {
		t.Errorf("hand-written lines must be kept verbatim at the top:\n%s", crontab)
	}
	if !strings.Contains(crontab, "@id:abc12345") {
		t.Errorf("new job missing:\n%s", crontab)
	}
}

func TestFileBackendWriteKeepsForeignLines(t *testing.T) {
	cronFile := filepath.Join(t.TempDir(), "crontab")
	if err := os.WriteFile(cronFile, []byte(handWritten), 0o644); err != nil {
		t.Fatal(err)
	}
	fb := NewFileBackend(cronFile, t.TempDir())
	if err := fb.WriteJobs([]cron.Job{newJob}); err != nil {
		t.Fatalf("WriteJobs: %v", err)
	}
	data, err := os.ReadFile(cronFile)
	if err != nil {
		t.Fatal(err)
	}
	assertKeepsHandWritten(t, string(data))
}

func TestRemoteCrontabKeepsForeignLines(t *testing.T) {
	got := remoteCrontab(handWritten, []cron.Job{newJob}, cron.DefaultRunner, "/home/srv")
	assertKeepsHandWritten(t, got)
	if !strings.Contains(got, "/home/srv/.lazycron/") {
		t.Errorf("job paths must be relocated to the server home:\n%s", got)
	}
}
