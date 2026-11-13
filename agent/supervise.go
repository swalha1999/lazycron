package agent

import "fmt"

// statusLost is recorded when `hc wait` kept failing and the session was
// stopped unseen: how far the agent got is unknown.
const statusLost = "lost"

// tracker keeps one run's session record up to date in the runs dir, where
// `lazycron exec` picks it up for the history entry.
type tracker struct {
	runsDir string
	runID   string // "" for an agent run outside a lazycron run: nothing is saved
	rec     SessionRecord
}

// save records the session's status. A record that can't be written only
// costs the run its `lazycron resume`, so the run itself goes on.
func (t *tracker) save(status string) {
	t.rec.Status = status
	_ = writeRecord(t.runsDir, t.runID, t.rec)
}

// continueHint tells a person how to pick the session up again.
func (t *tracker) continueHint() string {
	if t.runID != "" {
		return "continue it with `lazycron resume " + t.runID + "`"
	}
	return fmt.Sprintf("continue it with `hc attach %s`, or `claude --resume %s` in %s",
		t.rec.Terminal, t.rec.SessionID, t.rec.Dir)
}

// supervise starts the session, follows it to the end and records how it
// ended. The worktree is always kept once the session started: the session
// can be resumed there until retention removes it.
func supervise(sess *Session, t *tracker, jobName string) error {
	addr, err := sess.Start()
	if err != nil {
		if rmErr := RemoveWorktree(t.rec.Repo, t.rec.Dir); rmErr != nil {
			sess.Log("warning: could not remove worktree %s: %v", t.rec.Dir, rmErr)
		}
		return err
	}
	t.rec.Terminal = addr
	t.save("working")
	sess.Log("agent %q started in terminal %s (watch: hc attach %s)", jobName, addr, addr)
	sess.Log("claude session %s in %s", t.rec.SessionID, t.rec.Dir)

	out, err := sess.Wait(addr)
	if err != nil {
		return abandon(sess, t, err)
	}
	t.save(out.Status)
	return finish(out, t.continueHint(), sess.Log)
}

// abandon stops an agent lazycron lost track of, so it can't keep running
// unsupervised past its timeout. Its worktree and session record are kept:
// whatever it had done is still there to resume. When it can't be stopped
// the error says how to stop it by hand.
func abandon(sess *Session, t *tracker, cause error) error {
	t.save(statusLost)
	addr := t.rec.Terminal
	if _, err := sess.HC.Run("kill", addr); err != nil {
		return fmt.Errorf("lost track of agent terminal %s and could not stop it (%v); stop it with `hc kill %s`, its worktree is %s: %w",
			addr, err, addr, t.rec.Dir, cause)
	}
	return fmt.Errorf("lost track of agent terminal %s and stopped it; %s: %w", addr, t.continueHint(), cause)
}

// finish reports how a followed session ended. Its terminal stays open,
// except after a timeout, which stopped it.
func finish(out Outcome, continueHint string, log func(format string, args ...any)) error {
	switch out.Status {
	case "blocked":
		log("agent is waiting for input")
		return fmt.Errorf("%w: answer it in herdr-app or with `hc attach %s`", ErrBlocked, out.Terminal)
	case "done":
		log("agent finished; its session stays open: %s", continueHint)
	case "exited":
		log("claude exited with code %d; %s", out.ExitCode, continueHint)
	default:
		log("agent stopped: %s; %s", out.Status, continueHint)
	}
	switch {
	case out.Status == "exited" && out.ExitCode != 0:
		return fmt.Errorf("claude exited with code %d", out.ExitCode)
	case out.Status == "error", out.Status == "timeout":
		return fmt.Errorf("agent %s (read the session with `hc read %s`)", out.Status, out.Terminal)
	}
	return nil
}
