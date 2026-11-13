package cron

import "strings"

// ForeignLines returns the lines of crontab content that are not lazycron
// jobs (env assignments like MAILTO=, @reboot/@daily entries, unrelated
// comments), verbatim and in their original order.
func ForeignLines(existing string) []string {
	return parseCrontab(existing, ScriptsDir()).foreign
}

// KeepForeignLines puts the foreign lines of the existing crontab ahead of
// formatted job content, so rewriting the crontab never drops them. They go
// first so env assignments keep applying to the jobs after them.
func KeepForeignLines(existing, jobsContent string) string {
	foreign := ForeignLines(existing)
	if len(foreign) == 0 {
		return jobsContent
	}
	return strings.Join(foreign, "\n") + "\n" + jobsContent
}
