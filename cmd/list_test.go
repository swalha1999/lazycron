package cmd

import (
	"testing"

	"github.com/swalha1999/lazycron/cron"
)

func TestJobStatus(t *testing.T) {
	ref := "bash '/home/u/.lazycron/scripts/abc12345.sh'"
	tests := []struct {
		job  cron.Job
		want string
	}{
		{cron.Job{Enabled: true, Command: "echo"}, "enabled"},
		{cron.Job{Enabled: false, Command: "echo"}, "disabled"},
		{cron.Job{Enabled: true, Command: ref}, "enabled,script-missing"},
	}
	for _, tt := range tests {
		if got := jobStatus(tt.job); got != tt.want {
			t.Errorf("jobStatus(%q) = %q, want %q", tt.job.Command, got, tt.want)
		}
	}
	if !toJobView(cron.Job{Command: ref}).ScriptMissing {
		t.Error("JSON view should report script_missing")
	}
}
