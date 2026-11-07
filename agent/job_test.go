package agent

import (
	"strings"
	"testing"
	"time"
)

func TestParseJob_Full(t *testing.T) {
	data := []byte("\xef\xbb\xbf---\nname: Audit\ncron: every sunday at 3am\nmodel: opus\npermission_mode: bypassPermissions\ntimeout: 45m\ntag: AU\n---\n\nAudit the repo.\n\nFile issues.\n")
	j, err := ParseJob(data)
	if err != nil {
		t.Fatalf("ParseJob: %v", err)
	}
	if j.Name != "Audit" || j.Cron != "every sunday at 3am" || j.Model != "opus" || j.Tag != "AU" {
		t.Errorf("front matter not parsed: %+v", j)
	}
	if j.Prompt != "Audit the repo.\n\nFile issues." {
		t.Errorf("Prompt = %q", j.Prompt)
	}
	if j.PermissionModeOrDefault() != "bypassPermissions" {
		t.Errorf("permission mode = %q", j.PermissionModeOrDefault())
	}
	if d, _ := j.TimeoutDuration(); d != 45*time.Minute {
		t.Errorf("timeout = %v", d)
	}
}

func TestParseJob_Defaults(t *testing.T) {
	j, err := ParseJob([]byte("---\nname: W\ncron: every 4 hours\n---\nwork"))
	if err != nil {
		t.Fatalf("ParseJob: %v", err)
	}
	if j.PermissionModeOrDefault() != DefaultPermissionMode {
		t.Errorf("default permission mode = %q", j.PermissionModeOrDefault())
	}
	if d, _ := j.TimeoutDuration(); d != DefaultTimeout {
		t.Errorf("default timeout = %v", d)
	}
}

func TestParseJob_Errors(t *testing.T) {
	cases := map[string]string{
		"no front matter": "just a prompt",
		"unterminated":    "---\nname: X\ncron: every hour\n",
		"missing name":    "---\ncron: every hour\n---\nwork",
		"missing cron":    "---\nname: X\n---\nwork",
		"empty prompt":    "---\nname: X\ncron: every hour\n---\n   \n",
		"bad timeout":     "---\nname: X\ncron: every hour\ntimeout: soon\n---\nwork",
		"bad yaml":        "---\nname: [\n---\nwork",
	}
	for name, src := range cases {
		if _, err := ParseJob([]byte(src)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestBuildPrompt(t *testing.T) {
	p := BuildPrompt(Job{Prompt: "Do X."}, PromptContext{
		JobName: "Worker", Repo: "me/repo", Base: "origin/main", Worktree: "/wt", Notes: "Run pnpm test.",
	})
	for _, want := range []string{"job: Worker", "me/repo", "/wt", "origin/main", "Run pnpm test.", "## Task\n\nDo X."} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt missing %q:\n%s", want, p)
		}
	}
	if strings.Contains(BuildPrompt(Job{Prompt: "x"}, PromptContext{}), "Notes about") {
		t.Error("empty notes should not add a notes section")
	}
}

func TestTerminalName(t *testing.T) {
	got := terminalName("Upstream_Sync.very-long-name", "20261107-153000")
	if got != "agent-upstream_sync-1107153000" {
		t.Errorf("terminalName = %q", got)
	}
	if len(got) > 32 {
		t.Errorf("terminal name longer than hc allows: %d", len(got))
	}
}

func TestAgentPATH(t *testing.T) {
	got := agentPATH("/h", "/usr/bin:/custom")
	if !strings.HasPrefix(got, "/usr/bin:/custom:") {
		t.Errorf("existing PATH must stay first: %q", got)
	}
	for _, want := range []string{"/h/.local/bin", "/h/.local/share/mise/shims"} {
		if !strings.Contains(got, want) {
			t.Errorf("PATH missing %s: %q", want, got)
		}
	}
	if strings.Count(got, "/usr/bin:") != 1 {
		t.Errorf("duplicate entries: %q", got)
	}
}
