// Package agent runs scheduled Claude Code agents: real interactive sessions
// in herdr terminals, on this machine, in a fresh git worktree per run.
//
// A job is a Markdown file in .lazycron/jobs/ — YAML front matter for the
// schedule and session options, the body is the prompt:
//
//	---
//	name: Audit
//	cron: every sunday at 3am
//	model: opus
//	---
//	Audit this repository and file issues for ...
package agent

import (
	"bytes"
	"fmt"
	"time"

	"github.com/swalha1999/lazycron/cron"
	"gopkg.in/yaml.v3"
)

// Defaults applied when a job's front matter leaves a field out.
const (
	DefaultPermissionMode = "auto"
	DefaultTimeout        = 2 * time.Hour
)

// Job is one agent job file.
type Job struct {
	Name           string `yaml:"name"`
	Cron           string `yaml:"cron"`
	Description    string `yaml:"description"`
	Tag            string `yaml:"tag"`
	TagColor       string `yaml:"tag_color"`
	Model          string `yaml:"model"`           // passed to claude --model; empty = claude's default
	PermissionMode string `yaml:"permission_mode"` // claude --permission-mode (auto, bypassPermissions, ...)
	Base           string `yaml:"base"`            // branch to start from; empty = origin's default branch
	Timeout        string `yaml:"timeout"`         // Go duration, e.g. 90m; the session is stopped after it

	// Prompt is the Markdown body after the front matter.
	Prompt string `yaml:"-"`
}

var frontMatterFence = []byte("---")

// ParseJob parses a job file. `name`, `cron` and a non-empty prompt are required.
func ParseJob(data []byte) (Job, error) {
	var j Job
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")) // UTF-8 BOM
	data = bytes.TrimLeft(data, " \t\r\n")
	if !bytes.HasPrefix(data, frontMatterFence) {
		return j, fmt.Errorf("missing front matter: the file must start with a --- block")
	}
	rest := data[len(frontMatterFence):]
	end := bytes.Index(rest, append([]byte("\n"), frontMatterFence...))
	if end < 0 {
		return j, fmt.Errorf("unterminated front matter: no closing ---")
	}
	if err := yaml.Unmarshal(rest[:end], &j); err != nil {
		return j, fmt.Errorf("invalid front matter: %w", err)
	}
	body := rest[end+1+len(frontMatterFence):]
	j.Prompt = string(bytes.TrimSpace(body))

	switch {
	case j.Name == "":
		return j, fmt.Errorf("front matter missing required field: name")
	case j.Cron == "":
		return j, fmt.Errorf("front matter missing required field: cron")
	case j.Prompt == "":
		return j, fmt.Errorf("job has no prompt after the front matter")
	}
	if _, err := j.TimeoutDuration(); err != nil {
		return j, err
	}
	return j, j.validateLabels()
}

// validateLabels rejects a name, tag or tag color that cannot be written
// safely into a crontab line.
func (j Job) validateLabels() error {
	return cron.Job{Name: j.Name, Tag: j.Tag, TagColor: j.TagColor}.ValidateLabels()
}

// PermissionModeOrDefault returns the configured permission mode or the default.
func (j Job) PermissionModeOrDefault() string {
	if j.PermissionMode == "" {
		return DefaultPermissionMode
	}
	return j.PermissionMode
}

// TimeoutDuration returns the session time limit.
func (j Job) TimeoutDuration() (time.Duration, error) {
	if j.Timeout == "" {
		return DefaultTimeout, nil
	}
	d, err := time.ParseDuration(j.Timeout)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("invalid timeout %q: use a duration like 90m or 2h", j.Timeout)
	}
	return d, nil
}
