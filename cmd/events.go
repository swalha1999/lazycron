package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/swalha1999/lazycron/backend"
)

var eventsCmd = &cobra.Command{
	Use:   "events",
	Short: "Follow runs as they start and finish (one JSON object per line)",
	Long: "Prints run.started, run.finished (succeeded) and run.failed events as they " +
		"happen, one JSON object per line, until interrupted. Runs that already exist when " +
		"it starts are not reported. Works for plain and herdr runs.",
	Args: cobra.NoArgs,
	RunE: runEvents,
}

var (
	eventsJob   string
	eventsCount int
	eventsFor   time.Duration
)

func init() {
	eventsCmd.Flags().StringVar(&eventsJob, "job", "", "only events of this job (id or name)")
	eventsCmd.Flags().IntVar(&eventsCount, "count", 0, "exit after this many events (0: never)")
	eventsCmd.Flags().DurationVar(&eventsFor, "for", 0, "exit after this long, e.g. 1h (0: never)")
	rootCmd.AddCommand(eventsCmd)
}

// Event types.
const (
	eventStarted  = "run.started"
	eventFinished = "run.finished"
	eventFailed   = "run.failed"
)

// runEvent is one line of `events` output.
type runEvent struct {
	Type string  `json:"type"`
	At   string  `json:"at"`
	Run  runView `json:"run"`
}

// diffRuns compares the previous run states with the current runs and
// returns the events between them plus the new states. A run seen for the
// first time already finished yields both started and finished events.
func diffRuns(prev map[string]string, runs []runView, now time.Time) ([]runEvent, map[string]string) {
	next := make(map[string]string, len(runs))
	var events []runEvent
	at := now.Format(time.RFC3339)
	// oldest first, so events come out in the order they happened
	for i := len(runs) - 1; i >= 0; i-- {
		r := runs[i]
		next[r.ID] = r.State
		old, seen := prev[r.ID]
		if !seen {
			events = append(events, runEvent{Type: eventStarted, At: at, Run: r})
		}
		if r.finished() && (!seen || old == stateRunning) {
			typ := eventFinished
			if r.State != stateSucceeded {
				typ = eventFailed
			}
			events = append(events, runEvent{Type: typ, At: at, Run: r})
		}
	}
	return events, next
}

func runEvents(cmd *cobra.Command, args []string) error {
	return withBackend(func(b backend.Backend) error {
		jobID, err := eventsJobID(b)
		if err != nil {
			return err
		}
		runs, err := listRuns(b, jobID)
		if err != nil {
			return err
		}
		_, state := diffRuns(nil, runs, time.Now())
		var deadline time.Time
		if eventsFor > 0 {
			deadline = time.Now().Add(eventsFor)
		}
		emitted := 0
		for deadline.IsZero() || time.Now().Before(deadline) {
			time.Sleep(pollInterval(b))
			if runs, err = listRuns(b, jobID); err != nil {
				return err
			}
			var events []runEvent
			events, state = diffRuns(state, runs, time.Now())
			for _, ev := range events {
				if err := printEvent(cmd, ev); err != nil {
					return err
				}
				if emitted++; eventsCount > 0 && emitted >= eventsCount {
					return nil
				}
			}
		}
		return nil
	})
}

func eventsJobID(b backend.Backend) (string, error) {
	if eventsJob == "" {
		return "", nil
	}
	jobs, err := b.ReadJobs()
	if err != nil {
		return "", errUnavailable("read jobs: %v", err)
	}
	i, err := findJob(jobs, eventsJob)
	if err != nil {
		return "", err
	}
	return jobs[i].ID, nil
}

// printEvent writes one event: a compact JSON line, or a text line.
func printEvent(cmd *cobra.Command, ev runEvent) error {
	w := cmd.OutOrStdout()
	if wantJSON(cmd) {
		data, err := jsonLine(ev)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	}
	_, err := fmt.Fprintf(w, "%s  %-12s %s (%s) %s\n", ev.At, ev.Type, ev.Run.JobName, ev.Run.ID, ev.Run.State)
	return err
}
