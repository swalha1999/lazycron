package backend

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/swalha1999/lazycron/config"
	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/history"
	"github.com/swalha1999/lazycron/runner"
)

// ExecRequest describes a run started through a backend.
type ExecRequest struct {
	RunID   string // optional; generated when empty
	Trigger string // runner.TriggerManual or runner.TriggerCron
	Once    bool   // disable the job after the run
}

// execLocally runs job on this machine through the run engine, with paths
// for scripts/history/runs, and returns the recorded history entry.
func execLocally(b Backend, paths runner.Paths, job cron.Job, req ExecRequest) (history.Entry, error) {
	if err := ensureScript(paths, job); err != nil {
		return history.Entry{}, err
	}
	cfg, err := config.Load()
	switch {
	case err != nil:
		fmt.Fprintf(os.Stderr, "lazycron: using default settings, config unreadable: %v\n", err)
		cfg = &config.Config{}
	case cfg.EnsureSpaceID():
		_ = config.Save(cfg) // keep the herdr-app space stable across runs
	}
	e := runner.NewExecutor(paths, cfg)
	e.DisableOnce = func(jobID string) error { return setJobEnabled(b, jobID, false) }
	return e.Run(runner.Request{
		JobID:   job.ID,
		JobName: job.Name,
		Runner:  job.EffectiveRunner(),
		Trigger: req.Trigger,
		Once:    req.Once,
		RunID:   req.RunID,
	})
}

// ensureScript writes the job's script if it is missing (sync normally
// owns it). A script-ref command is never written over its own file.
func ensureScript(paths runner.Paths, job cron.Job) error {
	if _, err := os.Stat(paths.ScriptPath(job.ID)); err == nil {
		return nil
	}
	if job.Command == "" || cron.IsScriptRef(job.Command) {
		return fmt.Errorf("script %s is missing (re-save the job to recreate it)", paths.ScriptPath(job.ID))
	}
	return cron.WriteScriptIn(paths.Scripts, job.ID, job.Command)
}

// setJobEnabled flips one job's enabled flag in b's crontab.
func setJobEnabled(b Backend, jobID string, enabled bool) error {
	jobs, err := b.ReadJobs()
	if err != nil {
		return err
	}
	for i := range jobs {
		if jobs[i].ID == jobID {
			jobs[i].Enabled = enabled
			return b.WriteJobs(jobs)
		}
	}
	return fmt.Errorf("job %s not found", jobID)
}

// startLocally ensures the script exists and starts a detached `lazycron
// exec` for the run; extraArgs go before the subcommand (e.g. --cron-file).
func startLocally(paths runner.Paths, job cron.Job, req ExecRequest, extraArgs ...string) error {
	if err := ensureScript(paths, job); err != nil {
		return err
	}
	return SpawnDetached(append(extraArgs, execArgs(job, req)...))
}

// SpawnDetached starts `lazycron <args>` in its own session with no stdio,
// so it outlives the caller. Tests replace it.
var SpawnDetached = func(args []string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	c := exec.Command(exe, args...)
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := c.Start(); err != nil {
		return err
	}
	return c.Process.Release()
}

// execArgs builds the `lazycron exec` arguments for a run.
func execArgs(job cron.Job, req ExecRequest) []string {
	args := []string{"exec", job.ID, "--name", job.Name, "--runner", job.EffectiveRunner(), "--trigger", req.Trigger}
	if req.RunID != "" {
		args = append(args, "--run-id", req.RunID)
	}
	if req.Once {
		args = append(args, "--once")
	}
	return args
}

// localHC returns the herdr-core client for this machine.
func localHC() (*runner.HC, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	bin, err := runner.FindHC(cfg.Herdr.Bin)
	if err != nil {
		return nil, err
	}
	return &runner.HC{Bin: bin, Cmd: runner.LocalCommander{}}, nil
}

// localCommand returns a command attached to this terminal's stdio.
func localCommand(argv []string) *exec.Cmd {
	c := exec.Command(argv[0], argv[1:]...)
	c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
	return c
}

// killGroup sends SIGTERM to a local process group.
func killGroup(pgid int) error {
	if pgid <= 0 {
		return fmt.Errorf("invalid process group %d", pgid)
	}
	return syscall.Kill(-pgid, syscall.SIGTERM)
}
