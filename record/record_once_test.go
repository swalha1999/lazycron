package record

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fakeCrontabScript stands in for crontab(1): `-l` prints $CRONTAB_FILE
// (failing when it is missing) and `-` replaces it with stdin.
const fakeCrontabScript = `#!/bin/sh
case "$1" in
  -l) cat "$CRONTAB_FILE" ;;
  -) cat > "$CRONTAB_FILE" ;;
esac
`

// runOnce runs the record script with --once for jobID against a fake
// crontab holding initial ("" = no crontab) and returns the crontab after.
func runOnce(t *testing.T, jobID, initial string) string {
	t.Helper()
	dir := t.TempDir()
	crontabFile := filepath.Join(dir, "crontab.txt")
	if initial != "" {
		writeFile(t, crontabFile, initial, 0o600)
	}
	binDir := filepath.Join(dir, "bin")
	if err := os.Mkdir(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(binDir, "crontab"), fakeCrontabScript, 0o755)
	scriptPath := filepath.Join(dir, "record")
	writeFile(t, scriptPath, string(ScriptContent), 0o755)

	cmd := exec.Command("sh", scriptPath, jobID, "job", "0", "--once")
	cmd.Stdin = strings.NewReader("output")
	cmd.Env = []string{
		"HOME=" + dir,
		"PATH=" + binDir + string(os.PathListSeparator) + os.Getenv("PATH"),
		"CRONTAB_FILE=" + crontabFile,
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("script failed: %v\n%s", err, out)
	}
	data, err := os.ReadFile(crontabFile)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(data)
}

func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func TestScriptOnce_DisablesOnlyExactID(t *testing.T) {
	initial := "# a @id:fix @once\n0 1 * * * x\n" +
		"# b @id:fix-agent\n0 2 * * * y\n" +
		"# c @id:fix2\n0 3 * * * z\n" +
		"# d @id:fix_x [tag:#fff]\n0 4 * * * w\n" +
		"# foreign note mentioning @id:fix\n0 5 * * * v\n"
	want := "# a @id:fix @once\n#DISABLED 0 1 * * * x\n" +
		"# b @id:fix-agent\n0 2 * * * y\n" +
		"# c @id:fix2\n0 3 * * * z\n" +
		"# d @id:fix_x [tag:#fff]\n0 4 * * * w\n" +
		"# foreign note mentioning @id:fix\n#DISABLED 0 5 * * * v\n"
	// The foreign comment is a valid name comment for id "fix" by
	// cron.extractID's rules, so it is disabled too; only prefix ids
	// (fix-agent, fix2, fix_x) must stay enabled.
	if got := runOnce(t, "fix", initial); got != want {
		t.Errorf("crontab after --once:\n%s\nwant:\n%s", got, want)
	}
}

func TestScriptOnce_MatchesIDFollowedByMarkers(t *testing.T) {
	initial := "# a @id:fix [t:#fff] {proj}\n0 1 * * * x\n"
	want := "# a @id:fix [t:#fff] {proj}\n#DISABLED 0 1 * * * x\n"
	if got := runOnce(t, "fix", initial); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestScriptOnce_NoDoublePrefix(t *testing.T) {
	initial := "# a @id:fix @once\n#DISABLED 0 1 * * * x\n"
	if got := runOnce(t, "fix", initial); got != initial {
		t.Errorf("got:\n%s\nwant unchanged:\n%s", got, initial)
	}
}

func TestScriptOnce_MissingCrontabNotCreated(t *testing.T) {
	if got := runOnce(t, "fix", ""); got != "" {
		t.Errorf("crontab written from nothing: %q", got)
	}
}

func TestScriptOnce_EmptyIDTouchesNothing(t *testing.T) {
	initial := "# a @id:fix\n0 1 * * * x\n"
	if got := runOnce(t, "", initial); got != initial {
		t.Errorf("got:\n%s\nwant unchanged:\n%s", got, initial)
	}
}
