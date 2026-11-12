package cmd

import (
	"fmt"
	"os"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
	"github.com/swalha1999/lazycron/backend"
	"github.com/swalha1999/lazycron/runner"
)

var attachCmd = &cobra.Command{
	Use:   "attach <run-or-job>",
	Short: "Take over a herdr run's terminal (detach with ctrl+\\)",
	Long: "Puts this terminal on the run's herdr-core terminal, live or held after it " +
		"finished, so you can watch it or answer a blocked agent. For an agent job that is " +
		"its Claude session; `lazycron resume` also reopens one whose terminal was closed. " +
		"Needs a TTY; agents should use `logs` and `wait` instead.",
	Args: cobra.ExactArgs(1),
	RunE: runAttach,
}

var attachObserve bool

func init() {
	attachCmd.Flags().BoolVar(&attachObserve, "observe", false, "watch only: never type or resize")
	rootCmd.AddCommand(attachCmd)
}

func runAttach(cmd *cobra.Command, args []string) error {
	if !isatty.IsTerminal(os.Stdin.Fd()) {
		return errUsage("attach needs an interactive terminal").withHint("use `lazycron logs <run>` to read output, `lazycron wait <run>` to wait")
	}
	return withBackend(func(b backend.Backend) error {
		run, _, err := resolveRun(b, args[0])
		if err != nil {
			return err
		}
		if run.Terminal == "" {
			return errConflict("run %s did not run in a herdr terminal", run.ID).withHint("see its output with `lazycron logs %s`", run.ID)
		}
		hc, err := b.Herdr()
		if err != nil {
			return errUnavailable("%v", err)
		}
		addr := run.Terminal
		if t, ok := agentTerminal(hc, run); ok {
			addr = t.Address
		} else if _, err := hc.Get(addr); err != nil {
			hint := fmt.Sprintf("read the recorded output with `lazycron logs %s`", run.ID)
			if run.Agent != nil && run.Agent.SessionID != "" {
				hint = fmt.Sprintf("reopen its Claude session with `lazycron resume %s`", run.ID)
			}
			return errNotFound("terminal %s of run %s is gone", addr, run.ID).withHint("%s", hint)
		}
		return attachTo(b, hc, addr)
	})
}

// attachTo puts this terminal on addr.
func attachTo(b backend.Backend, hc *runner.HC, addr string) error {
	argv := hc.AttachArgs(addr)
	if attachObserve {
		argv = append(argv, "--observe")
	}
	c, err := b.InteractiveCommand(argv)
	if err != nil {
		return errFailure("%v", err)
	}
	if err := c.Run(); err != nil {
		return errFailure("attach: %v", err)
	}
	return nil
}
