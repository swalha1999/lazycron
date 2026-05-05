package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/swalha1999/lazycron/cron"
)

// --- filterByProject ---

func TestFilterByProject(t *testing.T) {
	jobs := []cron.Job{
		{ID: "a", Project: "backend"},
		{ID: "b", Project: "frontend"},
		{ID: "c", Project: "backend"},
		{ID: "d", Project: ""},
	}

	filtered := filterByProject(jobs, "backend")
	if len(filtered) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(filtered))
	}
	for _, j := range filtered {
		if j.Project != "backend" {
			t.Errorf("unexpected project %q", j.Project)
		}
	}
}

func TestFilterByProject_CaseSensitive(t *testing.T) {
	jobs := []cron.Job{
		{ID: "a", Project: "Backend"},
		{ID: "b", Project: "backend"},
	}

	filtered := filterByProject(jobs, "backend")
	if len(filtered) != 1 {
		t.Fatalf("expected 1 job (case-sensitive), got %d", len(filtered))
	}
	if filtered[0].ID != "b" {
		t.Errorf("wrong job filtered: %q", filtered[0].ID)
	}
}

func TestFilterByProject_NoMatch(t *testing.T) {
	jobs := []cron.Job{
		{ID: "a", Project: "backend"},
	}

	filtered := filterByProject(jobs, "nonexistent")
	if len(filtered) != 0 {
		t.Fatalf("expected 0 jobs, got %d", len(filtered))
	}
}

// --- writeJobStub ---

func TestWriteJobStub_BasicJob(t *testing.T) {
	dir := t.TempDir()
	job := cron.Job{
		ID:       "fix-agent",
		Name:     "Fix Agent",
		Schedule: "0 9 * * 1-5",
		Command:  "cd ~/.lazycron/projects/lazycron && .sandcastle/node_modules/.bin/tsx .sandcastle/jobs/fix-agent.ts",
		Enabled:  true,
		Project:  "lazycron",
		Tag:      "BP",
		TagColor: "#f38ba8",
	}

	if err := writeJobStub(dir, job); err != nil {
		t.Fatalf("writeJobStub: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "fix-agent.crontab"))
	if err != nil {
		t.Fatalf("read stub: %v", err)
	}
	got := string(data)

	wantSubstrings := []string{
		"# id: fix-agent",
		"# name: Fix Agent",
		"# schedule: 0 9 * * 1-5",
		"# project: lazycron",
		"# project_dir: ~/.lazycron/projects/lazycron",
		"# tag: BP",
		".sandcastle/node_modules/.bin/tsx .sandcastle/jobs/fix-agent.ts",
	}
	for _, s := range wantSubstrings {
		if !strings.Contains(got, s) {
			t.Errorf("stub missing %q\nfull stub:\n%s", s, got)
		}
	}

	// The cd prefix should NOT remain in the user-visible body line.
	bodyLines := strings.Split(strings.TrimSpace(got), "\n")
	body := bodyLines[len(bodyLines)-1]
	if strings.HasPrefix(body, "cd ") {
		t.Errorf("expected cd prefix stripped, body line: %q", body)
	}
}

func TestWriteJobStub_DisabledAndOnce(t *testing.T) {
	dir := t.TempDir()
	job := cron.Job{
		ID:       "one-shot",
		Name:     "OneShot",
		Schedule: "0 0 * * *",
		Command:  "echo bye",
		Enabled:  false,
		OneShot:  true,
	}
	if err := writeJobStub(dir, job); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "one-shot.crontab"))
	got := string(data)
	if !strings.Contains(got, "# enabled: false") {
		t.Error("disabled flag not in stub")
	}
	if !strings.Contains(got, "# once: true") {
		t.Error("once flag not in stub")
	}
}

func TestWriteJobStub_NoCdPrefixNoExtraField(t *testing.T) {
	dir := t.TempDir()
	job := cron.Job{
		ID:       "plain",
		Name:     "Plain",
		Schedule: "* * * * *",
		Command:  "echo hi",
		Enabled:  true,
	}
	if err := writeJobStub(dir, job); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "plain.crontab"))
	got := string(data)
	if strings.Contains(got, "# project_dir:") {
		t.Errorf("project_dir line should be absent when no cd prefix; got:\n%s", got)
	}
}
