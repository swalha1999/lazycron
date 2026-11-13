package runner

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/swalha1999/lazycron/config"
)

// runPlain runs the job's script as a child process, like cron would, and
// captures stdout and stderr together.
func (e *Executor) runPlain(a *Active) outcome {
	a.Runner = config.RunnerPlain
	var buf bytes.Buffer
	c := exec.Command("bash", e.Paths.ScriptPath(a.JobID))
	c.Dir, _ = os.UserHomeDir()
	c.Env = append(os.Environ(), e.Paths.runEnv(a.RunID)...)
	c.Stdout = &buf
	c.Stderr = &buf
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} // `kill` ends the whole job

	if err := c.Start(); err != nil {
		code := 127
		return outcome{exitCode: &code, output: "lazycron: could not start bash: " + err.Error()}
	}
	a.PID = c.Process.Pid
	e.markStarted(*a)

	err := c.Wait()
	out := outcome{output: strings.TrimRight(buf.String(), "\n")}
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		code := 0
		out.exitCode = &code
	case errors.As(err, &exitErr):
		if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
			out.signal = int(ws.Signal())
		} else {
			code := exitErr.ExitCode()
			out.exitCode = &code
		}
	default:
		out.note = "lazycron: " + err.Error()
	}
	return out
}
