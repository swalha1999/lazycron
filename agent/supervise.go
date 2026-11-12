package agent

import "fmt"

// workspace is the worktree one agent run works in.
type workspace struct {
	repo string // the repository the worktree belongs to
	dir  string // the worktree itself
}

// remove deletes the worktree, logging (not failing) when it can't.
func (w workspace) remove(log func(format string, args ...any)) {
	if err := RemoveWorktree(w.repo, w.dir); err != nil {
		log("warning: could not remove worktree %s: %v", w.dir, err)
	}
}

// supervise starts the session in ws, follows it to the end and cleans up.
func supervise(sess *Session, ws workspace, jobName string) error {
	addr, err := sess.Start()
	if err != nil {
		ws.remove(sess.Log)
		return err
	}
	sess.Log("agent %q started in terminal %s (watch: hc attach %s)", jobName, addr, addr)

	out, err := sess.Wait(addr)
	if err != nil {
		return abandon(sess, ws, addr, err)
	}
	return finish(out, ws, sess.Log)
}

// abandon stops an agent lazycron lost track of, so it can't keep running
// unsupervised past its timeout, and removes its worktree. When the agent
// can't be stopped its worktree is kept, since it may still be working there.
func abandon(sess *Session, ws workspace, addr string, cause error) error {
	if _, err := sess.HC.Run("kill", addr); err != nil {
		return fmt.Errorf("lost track of agent terminal %s and could not stop it (%v); stop it with `hc kill %s`, worktree kept at %s: %w",
			addr, err, addr, ws.dir, cause)
	}
	ws.remove(sess.Log)
	return fmt.Errorf("lost track of agent terminal %s, stopped it: %w", addr, cause)
}

// finish reports how a followed session ended and removes its worktree,
// unless the agent is waiting on a person.
func finish(out Outcome, ws workspace, log func(format string, args ...any)) error {
	switch out.Status {
	case "blocked":
		log("agent is waiting for input; worktree kept at %s", ws.dir)
		return fmt.Errorf("%w: answer it with `hc attach %s` or in herdr-app", ErrBlocked, out.Terminal)
	case "done":
		log("agent finished")
	case "exited":
		log("claude exited with code %d", out.ExitCode)
	default:
		log("agent stopped: %s", out.Status)
	}
	ws.remove(log)
	switch {
	case out.Status == "exited" && out.ExitCode != 0:
		return fmt.Errorf("claude exited with code %d", out.ExitCode)
	case out.Status == "error", out.Status == "timeout":
		return fmt.Errorf("agent %s (read the session with `hc read %s`)", out.Status, out.Terminal)
	}
	return nil
}
