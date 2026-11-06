package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

// jsonFlag backs the global --json flag.
var jsonFlag bool

// stdoutIsTerminal reports whether stdout is a terminal. A variable so tests
// can pin it.
var stdoutIsTerminal = func() bool {
	fd := os.Stdout.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

// wantJSON reports whether a command should print JSON: --json forces it,
// --json=false forbids it, a nil cmd means text, otherwise JSON is used when stdout is not a TTY.
func wantJSON(cmd *cobra.Command) bool {
	if cmd == nil { // called directly (tests), not through cobra
		return false
	}
	if f := cmd.Flags().Lookup("json"); f != nil && f.Changed {
		return jsonFlag
	}
	return !stdoutIsTerminal()
}

// writeJSON prints v as indented JSON followed by a newline.
func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// jsonLine encodes v as one compact JSON line (for streams like events).
func jsonLine(v any) ([]byte, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// render prints v as JSON in JSON mode, otherwise calls text to print a
// human-readable form to stdout.
func render(cmd *cobra.Command, v any, text func(w io.Writer) error) error {
	if wantJSON(cmd) {
		return writeJSON(cmd.OutOrStdout(), v)
	}
	return text(cmd.OutOrStdout())
}

// humanOut is where older commands write their human-readable progress:
// stdout in text mode, stderr in JSON mode (set before each command runs).
var humanOut io.Writer = os.Stdout

// progress returns where human progress messages go: stdout in text mode,
// stderr in JSON mode so stdout stays a single JSON document.
func progress(cmd *cobra.Command) io.Writer {
	if wantJSON(cmd) {
		return cmd.ErrOrStderr()
	}
	return cmd.OutOrStdout()
}

// printError reports err on stderr in the active output mode.
func printError(cmd *cobra.Command, w io.Writer, ce *cliError) {
	if wantJSON(cmd) {
		_ = writeJSON(w, map[string]*cliError{"error": ce})
		return
	}
	fmt.Fprintf(w, "Error: %s\n", ce.Message)
	if ce.Hint != "" {
		fmt.Fprintf(w, "Hint: %s\n", ce.Hint)
	}
}

// jsonForced reports whether --json was passed explicitly (for commands whose
// default output is not JSON even when piped, like exec and export).
func jsonForced(cmd *cobra.Command) bool {
	f := cmd.Flags().Lookup("json")
	return f != nil && f.Changed && jsonFlag
}
