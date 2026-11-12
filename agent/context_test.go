package agent

import "testing"

func TestCurrentRunPrefersRunIDFromEnv(t *testing.T) {
	t.Setenv("HC_TERMINAL_ID", "")
	t.Setenv("LAZYCRON_RUN_ID", "2026-11-12T20-00-00_job_abcd")
	if got := currentRun(nil).RunID; got != "2026-11-12T20-00-00_job_abcd" {
		t.Fatalf("RunID = %q", got)
	}
	t.Setenv("LAZYCRON_RUN_ID", "")
	if got := currentRun(nil); got != (runContext{}) {
		t.Fatalf("outside a run: %+v", got)
	}
}
