package cmd

import (
	"strings"
	"testing"
)

func TestLsEmptyIsJSONWhenPiped(t *testing.T) {
	env := newTestEnv(t)
	var out struct {
		Server string    `json:"server"`
		Jobs   []jobView `json:"jobs"`
	}
	env.mustRun("ls").decode(t, &out)
	if len(out.Jobs) != 0 || !strings.HasPrefix(out.Server, "file:") {
		t.Fatalf("unexpected: %+v", out)
	}
}

func TestJSONFalseForcesText(t *testing.T) {
	env := newTestEnv(t)
	r := env.mustRun("ls", "--json=false")
	if !strings.Contains(r.stdout, "No cron jobs found.") {
		t.Fatalf("expected text output, got %q", r.stdout)
	}
}

func TestErrorShapeAndExitCodes(t *testing.T) {
	env := newTestEnv(t)
	cases := []struct {
		args []string
		code int
		kind string
	}{
		{[]string{"no-such-command"}, exitUsage, kindUsage},
		{[]string{"ls", "--bogus"}, exitUsage, kindUsage},
		{[]string{"ls", "extra-arg"}, exitUsage, kindUsage},
		{[]string{"templates", "apply", "no-such-template"}, exitNotFound, kindNotFound},
	}
	for _, c := range cases {
		r := env.run(c.args...)
		if r.code != c.code {
			t.Errorf("%v: exit %d, want %d (stderr %s)", c.args, r.code, c.code, r.stderr)
			continue
		}
		if got := r.errorKind(t); got != c.kind {
			t.Errorf("%v: kind %q, want %q", c.args, got, c.kind)
		}
	}
}

func TestUnknownServerIsNotFound(t *testing.T) {
	newTestEnv(t)
	r := runCLI(t, "ls", "--server", "nope")
	if r.code != exitNotFound || r.errorKind(t) != kindNotFound {
		t.Fatalf("exit %d stderr %s", r.code, r.stderr)
	}
}

func TestTextErrorHasHint(t *testing.T) {
	env := newTestEnv(t)
	r := env.run("templates", "apply", "nope", "--json=false")
	if !strings.Contains(r.stderr, "Error: template \"nope\" not found") || !strings.Contains(r.stderr, "Hint:") {
		t.Fatalf("stderr = %q", r.stderr)
	}
}

func TestTemplatesLsJSON(t *testing.T) {
	env := newTestEnv(t)
	var out struct {
		Templates []templateView `json:"templates"`
	}
	env.mustRun("templates", "ls").decode(t, &out)
	if len(out.Templates) == 0 {
		t.Fatal("expected builtin templates")
	}
}
