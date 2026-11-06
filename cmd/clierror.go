package cmd

import (
	"errors"
	"fmt"
	"strings"
)

// Exit codes. They are part of the CLI contract documented in
// `lazycron agent-help`; do not renumber.
const (
	exitOK          = 0
	exitFailure     = 1 // generic failure, including a job run that failed
	exitUsage       = 2 // bad flags or arguments
	exitNotFound    = 3 // job, run, server or template does not exist (same as hc)
	exitConflict    = 4 // the change conflicts with existing state
	exitUnavailable = 5 // crontab, herdr-core or a server cannot be reached
)

// Error kinds, reported as error.kind in JSON.
const (
	kindFailure     = "failure"
	kindUsage       = "usage"
	kindNotFound    = "not_found"
	kindConflict    = "conflict"
	kindUnavailable = "unavailable"
	kindJobFailed   = "job_failed"
)

// cliError is an error with a machine-readable kind, an optional hint and the
// process exit code it maps to. Its JSON shape matches herdr-core's:
// {"error":{"kind","message","hint"}}.
type cliError struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
	Hint    string `json:"hint,omitempty"`
	code    int
}

func (e *cliError) Error() string { return e.Message }

// withHint returns e with its hint set, for chaining at the error site.
func (e *cliError) withHint(format string, args ...any) *cliError {
	e.Hint = fmt.Sprintf(format, args...)
	return e
}

func newCLIError(kind string, code int, format string, args ...any) *cliError {
	return &cliError{Kind: kind, Message: fmt.Sprintf(format, args...), code: code}
}

func errUsage(format string, args ...any) *cliError {
	return newCLIError(kindUsage, exitUsage, format, args...)
}

func errNotFound(format string, args ...any) *cliError {
	return newCLIError(kindNotFound, exitNotFound, format, args...)
}

func errConflict(format string, args ...any) *cliError {
	return newCLIError(kindConflict, exitConflict, format, args...)
}

func errUnavailable(format string, args ...any) *cliError {
	return newCLIError(kindUnavailable, exitUnavailable, format, args...)
}

func errFailure(format string, args ...any) *cliError {
	return newCLIError(kindFailure, exitFailure, format, args...)
}

func errJobFailed(format string, args ...any) *cliError {
	return newCLIError(kindJobFailed, exitFailure, format, args...)
}

// usageErrorPrefixes are the messages cobra/pflag use for argument and flag
// problems; they carry no type, so they are recognised by text.
var usageErrorPrefixes = []string{
	"unknown command",
	"unknown flag",
	"unknown shorthand flag",
	"flag needs an argument",
	"invalid argument",
	"required flag",
	"accepts ",
	"requires at least",
	"requires at most",
	"bad flag syntax",
	"if any flags in the group",
}

// asCLIError converts any error returned by a command into a cliError.
func asCLIError(err error) *cliError {
	var ce *cliError
	if errors.As(err, &ce) {
		return ce
	}
	msg := err.Error()
	for _, p := range usageErrorPrefixes {
		if strings.HasPrefix(msg, p) {
			return errUsage("%s", msg).withHint("run `lazycron <command> --help`")
		}
	}
	return errFailure("%s", msg)
}
