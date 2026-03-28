package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"
)

// Action represents a single notification action.
type Action struct {
	Type string `yaml:"type" json:"type"`                     // "webhook", "command", "desktop"
	URL  string `yaml:"url,omitempty" json:"url,omitempty"`   // for webhook type
	Run  string `yaml:"run,omitempty" json:"run,omitempty"`   // for command type
}

// Config holds notification configuration.
type Config struct {
	OnFailure []Action `yaml:"on_failure,omitempty" json:"on_failure,omitempty"`
	OnSuccess []Action `yaml:"on_success,omitempty" json:"on_success,omitempty"`
}

// HasNotifications returns true if any notifications are configured.
func (c Config) HasNotifications() bool {
	return len(c.OnFailure) > 0 || len(c.OnSuccess) > 0
}

// TemplateData holds the variables available for template substitution.
type TemplateData struct {
	JobName  string
	Schedule string
	ExitCode int
	Output   string
	Server   string
	Timestamp string
}

// WebhookPayload is the JSON body sent to webhook URLs.
type WebhookPayload struct {
	JobName   string `json:"job_name"`
	Schedule  string `json:"schedule"`
	ExitCode  int    `json:"exit_code"`
	Output    string `json:"output"`
	Server    string `json:"server"`
	Timestamp string `json:"timestamp"`
}

// NotifyDir returns the path to the notification config directory.
// It is a variable so tests can override it.
var NotifyDir = func() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".lazycron", "notify")
}

// JobConfigPath returns the path to a job's notification config file.
func JobConfigPath(jobID string) string {
	return filepath.Join(NotifyDir(), jobID+".conf")
}

// HasJobConfig reports whether a notification config file exists for the given job.
func HasJobConfig(jobID string) bool {
	_, err := os.Stat(JobConfigPath(jobID))
	return err == nil
}

// WriteJobConfig writes a per-job notification config file in TSV format.
// The file is read by the notify shell script after job execution.
func WriteJobConfig(jobID, schedule string, cfg Config) error {
	if !cfg.HasNotifications() {
		// Remove any existing config file.
		path := JobConfigPath(jobID)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}

	dir := NotifyDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "meta\tschedule\t%s\n", schedule)
	for _, a := range cfg.OnFailure {
		writeActionLine(&b, "on_failure", a)
	}
	for _, a := range cfg.OnSuccess {
		writeActionLine(&b, "on_success", a)
	}

	return os.WriteFile(JobConfigPath(jobID), []byte(b.String()), 0o600)
}

func writeActionLine(b *strings.Builder, event string, a Action) {
	switch a.Type {
	case "webhook":
		fmt.Fprintf(b, "%s\twebhook\t%s\n", event, a.URL)
	case "command":
		fmt.Fprintf(b, "%s\tcommand\t%s\n", event, a.Run)
	case "desktop":
		fmt.Fprintf(b, "%s\tdesktop\t%s\n", event, a.Run)
	}
}

// RemoveJobConfig removes a job's notification config file.
func RemoveJobConfig(jobID string) error {
	err := os.Remove(JobConfigPath(jobID))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// SyncConfigs writes notification config files for jobs that have them,
// and removes orphaned config files for jobs that no longer exist.
func SyncConfigs(jobConfigs map[string]struct{ Schedule string; Cfg Config }) error {
	for jobID, jc := range jobConfigs {
		if err := WriteJobConfig(jobID, jc.Schedule, jc.Cfg); err != nil {
			return fmt.Errorf("write notify config for %s: %w", jobID, err)
		}
	}
	return nil
}

// Send dispatches notifications for a completed job from Go code.
// Notification errors are returned as a combined error but should not
// affect the job's own status.
func Send(cfg Config, data TemplateData) error {
	var actions []Action
	if data.ExitCode != 0 {
		actions = cfg.OnFailure
	} else {
		actions = cfg.OnSuccess
	}

	var errs []string
	for _, a := range actions {
		var err error
		switch a.Type {
		case "webhook":
			err = sendWebhook(a.URL, data)
		case "command":
			err = runCommand(a.Run, data)
		case "desktop":
			err = sendDesktop(a.Run, data)
		}
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s(%s): %v", a.Type, actionTarget(a), err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("notification errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

func actionTarget(a Action) string {
	switch a.Type {
	case "webhook":
		return a.URL
	case "command":
		return a.Run
	case "desktop":
		return "desktop"
	}
	return a.Type
}

func sendWebhook(url string, data TemplateData) error {
	output := data.Output
	if len(output) > 1000 {
		output = output[:1000]
	}

	payload := WebhookPayload{
		JobName:   data.JobName,
		Schedule:  data.Schedule,
		ExitCode:  data.ExitCode,
		Output:    output,
		Server:    data.Server,
		Timestamp: data.Timestamp,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

func runCommand(cmdTemplate string, data TemplateData) error {
	rendered, err := renderTemplate(cmdTemplate, data)
	if err != nil {
		return err
	}

	ctx := exec.Command("sh", "-c", rendered)
	ctx.Env = append(os.Environ(), templateEnvVars(data)...)
	return ctx.Run()
}

func sendDesktop(msgTemplate string, data TemplateData) error {
	msg := ""
	if msgTemplate != "" {
		var err error
		msg, err = renderTemplate(msgTemplate, data)
		if err != nil {
			return err
		}
	}
	if msg == "" {
		if data.ExitCode == 0 {
			msg = fmt.Sprintf("%s completed successfully", data.JobName)
		} else {
			msg = fmt.Sprintf("%s failed (exit %d)", data.JobName, data.ExitCode)
		}
	}

	switch runtime.GOOS {
	case "linux":
		return exec.Command("notify-send", "lazycron", msg).Run()
	case "darwin":
		script := fmt.Sprintf(`display notification %q with title "lazycron"`, msg)
		return exec.Command("osascript", "-e", script).Run()
	default:
		return fmt.Errorf("desktop notifications not supported on %s", runtime.GOOS)
	}
}

func renderTemplate(tmpl string, data TemplateData) (string, error) {
	t, err := template.New("notify").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("invalid template: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template execution: %w", err)
	}
	return buf.String(), nil
}

func templateEnvVars(data TemplateData) []string {
	return []string{
		"LC_JOB_NAME=" + data.JobName,
		"LC_SCHEDULE=" + data.Schedule,
		fmt.Sprintf("LC_EXIT_CODE=%d", data.ExitCode),
		"LC_OUTPUT=" + data.Output,
		"LC_SERVER=" + data.Server,
		"LC_TIMESTAMP=" + data.Timestamp,
	}
}

// Hostname returns the machine's hostname for use in notifications.
func Hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}
