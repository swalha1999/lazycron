package backend

import (
	"errors"
	"strings"
	"testing"
)

// recordingRunner records the commands and stdin installCrontab sends.
type recordingRunner struct {
	cmds   []string
	stdins []string
	out    string
	err    error
}

func (r *recordingRunner) RunWithStdin(cmd, stdin string) (string, error) {
	r.cmds = append(r.cmds, cmd)
	r.stdins = append(r.stdins, stdin)
	return r.out, r.err
}

func TestInstallCrontabSendsContentUnchangedOnStdin(t *testing.T) {
	content := "MAILTO=x\n" +
		"@reboot printf 'booted\\n' >> /var/log/boot.txt\n" +
		"5 4 * * * /usr/bin/foo \\c tail\t'it''s' \\\\ \\t\n" +
		"# job @id:abc\n" +
		"* * * * * bash x\n"
	r := &recordingRunner{}

	if err := installCrontab(r, content); err != nil {
		t.Fatalf("installCrontab: %v", err)
	}
	if len(r.cmds) != 1 || r.cmds[0] != "crontab -" {
		t.Fatalf("commands = %q, want exactly [\"crontab -\"]", r.cmds)
	}
	if r.stdins[0] != content {
		t.Errorf("stdin changed:\n got %q\nwant %q", r.stdins[0], content)
	}
	if strings.Contains(r.cmds[0], "echo") {
		t.Errorf("command must not pass the crontab through echo: %q", r.cmds[0])
	}
}

func TestInstallCrontabReportsCrontabError(t *testing.T) {
	r := &recordingRunner{out: `"-":3: bad minute`, err: errors.New("exit status 1")}

	err := installCrontab(r, "bad\n")
	if err == nil {
		t.Fatal("want an error when crontab - fails")
	}
	if !strings.Contains(err.Error(), "bad minute") {
		t.Errorf("error %q should include crontab's message", err)
	}
}
