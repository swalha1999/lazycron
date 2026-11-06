package backend

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/swalha1999/lazycron/config"
	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/history"
	"github.com/swalha1999/lazycron/runner"
	sshclient "github.com/swalha1999/lazycron/ssh"
)

// sshCommander runs programs on a remote host for runner.HC.
type sshCommander struct{ client *sshclient.Client }

func (s sshCommander) Output(args []string) ([]byte, []byte, error) {
	out, err := s.client.Run(shellJoin(args))
	if err != nil {
		return nil, []byte(out), err
	}
	return []byte(out), nil, nil
}

func shellJoin(args []string) string {
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = shellQuote(a)
	}
	return strings.Join(quoted, " ")
}

// remoteLazycron returns the remote ~/.lazycron/bin/lazycron link, or ""
// when lazycron is not installed there (`lazycron setup --server` links it).
func (b *RemoteBackend) remoteLazycron() (string, error) {
	lcDir, err := b.lazycronDir()
	if err != nil {
		return "", err
	}
	bin := lcDir + "/bin/lazycron"
	ok, err := b.client.FileExists(bin)
	if err != nil || !ok {
		return "", err
	}
	return bin, nil
}

// ExecJob runs the job on the server through the server's own lazycron
// (herdr or plain, recorded there). Without lazycron on the server it runs
// the script directly and records a plain run, as older versions did.
func (b *RemoteBackend) ExecJob(job cron.Job, req ExecRequest) (history.Entry, error) {
	bin, err := b.remoteLazycron()
	if err != nil {
		return history.Entry{}, err
	}
	if bin == "" {
		return b.execLegacy(job, req)
	}
	out, err := b.client.Run(shellJoin(append([]string{bin}, append(execArgs(job, req), "--json")...)))
	if err != nil {
		return history.Entry{}, fmt.Errorf("remote lazycron exec: %v: %s", err, out)
	}
	var e history.Entry
	if err := json.Unmarshal([]byte(out), &e); err != nil {
		return history.Entry{}, fmt.Errorf("remote lazycron exec: unexpected output: %s", out)
	}
	return e, nil
}

// execLegacy runs the job's script over SSH and records a plain run.
func (b *RemoteBackend) execLegacy(job cron.Job, req ExecRequest) (history.Entry, error) {
	start := time.Now()
	if req.RunID == "" {
		req.RunID = runner.NewRunID(job.ID, start)
	}
	out, runErr := b.RunJob(job.ID, job.Name, job.Command)
	success := runErr == nil
	e := history.Entry{
		JobID: job.ID, JobName: job.Name, Timestamp: time.Now().Format(time.RFC3339),
		Output: out, Success: &success, RunID: req.RunID, StartedAt: start.Format(time.RFC3339),
		Runner: config.RunnerPlain, Trigger: req.Trigger,
		FallbackReason: "lazycron is not installed on the server (run `lazycron setup --server " + b.name + "`)",
	}
	if success {
		code := 0
		e.ExitCode = &code
	}
	data, err := history.MarshalEntry(e)
	if err != nil {
		return e, err
	}
	lcDir, err := b.lazycronDir()
	if err != nil {
		return e, err
	}
	return e, b.client.Upload(string(data), lcDir+"/history/"+history.EntryFileName(e.RunID), 0o600)
}

// StartJob starts a background run with the server's lazycron.
func (b *RemoteBackend) StartJob(job cron.Job, req ExecRequest) error {
	bin, err := b.remoteLazycron()
	if err != nil {
		return err
	}
	if bin == "" {
		return fmt.Errorf("background runs on %s need lazycron there (run `lazycron setup --server %s`)", b.name, b.name)
	}
	cmd := "nohup " + shellJoin(append([]string{bin}, execArgs(job, req)...)) + " >/dev/null 2>&1 </dev/null &"
	_, err = b.client.Run(cmd)
	return err
}

// ActiveRuns reads the server's run markers. Liveness of their supervising
// process cannot be checked remotely, so Stale is never set.
func (b *RemoteBackend) ActiveRuns() ([]runner.Active, error) {
	lcDir, err := b.lazycronDir()
	if err != nil {
		return nil, err
	}
	files, err := b.client.ListFiles(lcDir+"/runs", "*.json")
	if err != nil {
		return nil, nil // no runs dir yet
	}
	var runs []runner.Active
	for _, f := range files {
		data, err := b.client.ReadFile(f)
		if err != nil {
			continue
		}
		if a, err := runner.ParseActive(data); err == nil {
			runs = append(runs, a)
		}
	}
	runner.SortActive(runs)
	return runs, nil
}

// Herdr returns a client for herdr-core on the server.
func (b *RemoteBackend) Herdr() (*runner.HC, error) {
	out, err := b.client.Run(`command -v herdr-core 2>/dev/null || { [ -x "$HOME/.local/bin/herdr-core" ] && echo "$HOME/.local/bin/herdr-core"; }`)
	bin := strings.TrimSpace(out)
	if err != nil || bin == "" {
		return nil, fmt.Errorf("herdr-core not found on %s", b.name)
	}
	return &runner.HC{Bin: bin, Cmd: sshCommander{b.client}}, nil
}

// InteractiveCommand runs argv on the server through the system ssh with a TTY.
func (b *RemoteBackend) InteractiveCommand(argv []string) (*exec.Cmd, error) {
	c := exec.Command("ssh", append(b.client.SSHArgs(), shellJoin(argv))...)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	return c, nil
}

// KillProcessGroup sends SIGTERM to a process group on the server.
func (b *RemoteBackend) KillProcessGroup(pgid int) error {
	if pgid <= 0 {
		return fmt.Errorf("invalid process group %d", pgid)
	}
	_, err := b.client.Run(fmt.Sprintf("kill -TERM -- -%d", pgid))
	return err
}
