package runner

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/swalha1999/lazycron/config"
)

// finishedMarker starts the line the wrapper prints when the job ended.
const finishedMarker = "[lazycron] finished:"

// holdWrapper runs inside the herdr terminal. It runs the job from $HOME
// (like cron) with the run's environment ($5, $6: see Paths.runEnv), writes the exit code for `lazycron exec`, then — when a hold
// is set — keeps the terminal live in the hold folder so herdr-app, which
// only adopts live terminals, can still show the run later. The folder's
// name ("Cron") is what herdr-app calls the space it creates.
const holdWrapper = `script=$1 exitfile=$2 holddir=$3 hold=$4
cd "$HOME" 2>/dev/null
env "$5" "$6" bash "$script"
ec=$?
printf '%s\n' "$ec" > "$exitfile.tmp" && mv -f "$exitfile.tmp" "$exitfile"
if [ "$hold" -gt 0 ] 2>/dev/null; then
  mkdir -p "$holddir" && cd "$holddir"
  printf '\n` + finishedMarker + ` exit %s at %s. Kept for viewing; close this pane when done.\n' "$ec" "$(date '+%Y-%m-%d %H:%M')"
  sleep "$hold"
fi
exit "$ec"
`

// maxReadLines is how much history `hc read` is asked for; herdr-core caps
// its scrollback well below this and reports complete=false when it does.
const maxReadLines = 100000

// terminalName builds a herdr terminal name (1-32 of a-z 0-9 - _).
func terminalName(jobID, runID string) string {
	suffix := "-" + shortSuffix(runID)
	base := "cron-" + jobID
	if max := 32 - len(suffix); len(base) > max {
		base = strings.TrimRight(base[:max], "-_")
	}
	return base + suffix
}

// runHerdr starts the job in a herdr terminal and waits for it. When the
// terminal cannot be started, fallback says why and the caller runs the job
// plainly instead.
func (e *Executor) runHerdr(a *Active) (out outcome, fallback string) {
	exitFile := e.Paths.ExitFile(a.RunID)
	if err := os.MkdirAll(e.Paths.Runs, 0o700); err != nil {
		return outcome{}, "cannot create runs dir: " + err.Error()
	}
	_ = os.Remove(exitFile)

	a.TerminalName = terminalName(a.JobID, a.RunID)
	env := e.Paths.runEnv(a.RunID)
	addr, err := e.HC.Start(StartOptions{
		Name:   a.TerminalName,
		Cwd:    homeDir(),
		Labels: e.runLabels(*a),
		Argv: []string{"bash", "-c", holdWrapper, "lazycron-run",
			e.Paths.ScriptPath(a.JobID), exitFile, e.Paths.Hold, strconv.Itoa(int(e.Hold.Seconds())),
			env[0], env[1]},
	})
	if err != nil {
		a.TerminalName = ""
		return outcome{}, "herdr-core could not start the run: " + err.Error()
	}
	a.Terminal = addr
	a.Runner = config.RunnerHerdr
	e.markStarted(*a)
	if e.NameSpace != nil {
		e.NameSpace()
	}

	out = e.awaitHerdr(addr, exitFile)
	if read, err := e.HC.Read(addr, maxReadLines); err == nil {
		out.output = stripTrailer(read.Text)
		out.outputTruncated = !read.Complete
	} else if out.note == "" {
		out.note = "lazycron: could not read the terminal: " + err.Error()
	}
	return out, ""
}

// awaitHerdr waits until the job wrote its exit code or the terminal ended.
// With a hold the terminal outlives the job, so the exit file is the signal;
// hc wait --output wakes us as soon as the wrapper prints its trailer.
func (e *Executor) awaitHerdr(addr, exitFile string) outcome {
	timeout := e.WaitTimeout
	if timeout <= 0 {
		timeout = 60
	}
	minPoll := e.MinPoll
	if minPoll <= 0 {
		minPoll = 2 * time.Second
	}
	lostAfter := e.LostAfter
	if lostAfter <= 0 {
		lostAfter = 15 * time.Minute
	}

	var failingSince time.Time
	for {
		if code, ok := readExitFile(exitFile); ok {
			return outcome{exitCode: &code}
		}
		began := time.Now()
		w, err := e.HC.Wait(addr, timeout, finishedMarker)
		switch {
		case err != nil && IsHCNotFound(err):
			return outcome{note: "lazycron: the run's terminal " + addr + " was removed before it finished"}
		case err != nil:
			if failingSince.IsZero() {
				failingSince = time.Now()
			}
			if time.Since(failingSince) > lostAfter {
				return outcome{note: fmt.Sprintf("lazycron: lost contact with herdr-core for %s: %v", lostAfter, err)}
			}
		case w.Ended:
			if code, ok := readExitFile(exitFile); ok {
				return outcome{exitCode: &code}
			}
			return endedWithoutExitFile(w)
		default:
			failingSince = time.Time{}
		}
		if gap := minPoll - time.Since(began); gap > 0 {
			time.Sleep(gap)
		}
	}
}

// endedWithoutExitFile describes a terminal that ended before the job did:
// killed (`lazycron kill`, `hc kill`, pane closed) or the wrapper failed.
func endedWithoutExitFile(w WaitResult) outcome {
	if w.Signal != nil {
		return outcome{signal: *w.Signal, note: fmt.Sprintf("lazycron: run killed (signal %d)", *w.Signal)}
	}
	return outcome{exitCode: w.ExitCode}
}

// stripTrailer removes the wrapper's "finished" line from captured output.
func stripTrailer(text string) string {
	if i := strings.LastIndex(text, finishedMarker); i >= 0 {
		text = text[:i]
	}
	return strings.TrimRight(text, "\n ")
}

func homeDir() string {
	home, _ := os.UserHomeDir()
	return home
}

// runLabels are the labels of a run's terminal: the herdr-app space, the job
// and the run, then the configured extra labels (an empty value removes one).
func (e *Executor) runLabels(a Active) map[string]string {
	labels := map[string]string{LabelSpace: e.SpaceID, LabelJob: a.JobID, LabelRun: a.RunID}
	for k, v := range e.ExtraLabels {
		labels[k] = v
	}
	for k, v := range labels {
		if v == "" {
			delete(labels, k)
		}
	}
	return labels
}
