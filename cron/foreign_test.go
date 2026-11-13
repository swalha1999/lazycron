package cron

import (
	"reflect"
	"strings"
	"testing"
)

// userCrontab is a hand-written crontab with lines lazycron does not manage.
const userCrontab = "MAILTO=me@example.com\n" +
	"PATH=/usr/local/bin:/usr/bin:/bin\n" +
	"CRON_TZ = Europe/Berlin\n" +
	"# my backup\n" +
	"0 2 * * * /usr/local/bin/backup.sh\n" +
	"@reboot /home/u/start-agent.sh\n" +
	"@daily /usr/bin/find /tmp -mtime +7 -delete\n" +
	"# old job, kept for reference\n" +
	"\n" +
	"#0 3 * * * /usr/bin/legacy\n"

var userForeign = []string{
	"MAILTO=me@example.com",
	"PATH=/usr/local/bin:/usr/bin:/bin",
	"CRON_TZ = Europe/Berlin",
	"@reboot /home/u/start-agent.sh",
	"@daily /usr/bin/find /tmp -mtime +7 -delete",
	"# old job, kept for reference",
	"#0 3 * * * /usr/bin/legacy",
}

func TestParseKeepsOnlyRealJobs(t *testing.T) {
	withFakeScriptsDir(t)
	jobs := Parse(userCrontab)
	if len(jobs) != 1 {
		t.Fatalf("got %d jobs, want only the backup job: %+v", len(jobs), jobs)
	}
	if j := jobs[0]; j.Name != "my backup" || j.Schedule != "0 2 * * *" || j.Command != "/usr/local/bin/backup.sh" {
		t.Errorf("backup job = %+v", j)
	}
}

func TestForeignLinesVerbatimInOrder(t *testing.T) {
	withFakeScriptsDir(t)
	if got := ForeignLines(userCrontab); !reflect.DeepEqual(got, userForeign) {
		t.Errorf("ForeignLines =\n%q\nwant\n%q", got, userForeign)
	}
}

func TestSpecialScheduleLinesAreNotJobs(t *testing.T) {
	withFakeScriptsDir(t)
	for _, line := range []string{
		"@reboot /start.sh",
		"@hourly a b c d e f g",
		"#DISABLED @weekly /usr/bin/find /tmp -mtime +7 -delete",
		"PATH=/a /b /c /d /e /f",
	} {
		if jobs := Parse(line); len(jobs) != 0 {
			t.Errorf("Parse(%q) = %+v, want no jobs", line, jobs)
		}
		if got := ForeignLines(line); !reflect.DeepEqual(got, []string{line}) {
			t.Errorf("ForeignLines(%q) = %q", line, got)
		}
	}
}

func TestKeepForeignLinesRoundTrip(t *testing.T) {
	withFakeScriptsDir(t)
	first := KeepForeignLines(userCrontab, FormatCrontab(Parse(userCrontab)))
	if !strings.HasPrefix(first, strings.Join(userForeign, "\n")+"\n") {
		t.Errorf("foreign lines must lead the crontab byte-for-byte:\n%s", first)
	}
	second := KeepForeignLines(first, FormatCrontab(Parse(first)))
	if second != first {
		t.Errorf("rewriting must be stable, got\n%s\nwant\n%s", second, first)
	}
}

func TestKeepForeignLinesNoneLeavesContent(t *testing.T) {
	withFakeScriptsDir(t)
	content := FormatCrontab([]Job{{ID: "abc12345", Name: "j", Schedule: "* * * * *", Command: "echo", Enabled: true}})
	if got := KeepForeignLines(content, content); got != content {
		t.Errorf("lazycron-only crontab changed:\n%s", got)
	}
}

func TestWriteCrontabKeepsForeignLines(t *testing.T) {
	withFakeScriptsDir(t)
	var written string
	withFakeCrontab(t, func(stdin string, args ...string) (string, error) {
		if args[0] == "-l" {
			return userCrontab, nil
		}
		written = stdin
		return "", nil
	})
	if err := WriteCrontab(Parse(userCrontab)); err != nil {
		t.Fatalf("WriteCrontab: %v", err)
	}
	for _, line := range userForeign {
		if !strings.Contains(written, line+"\n") {
			t.Errorf("written crontab lost %q:\n%s", line, written)
		}
	}
}
