package backend

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/history"
	"github.com/swalha1999/lazycron/record"
	sshclient "github.com/swalha1999/lazycron/ssh"
)

// RemoteBackend implements Backend over an SSH connection.
type RemoteBackend struct {
	name   string
	client *sshclient.Client
}

// NewRemoteBackend creates a new remote backend.
func NewRemoteBackend(name string, client *sshclient.Client) *RemoteBackend {
	return &RemoteBackend{name: name, client: client}
}

func (b *RemoteBackend) Name() string { return b.name }

func (b *RemoteBackend) ReadJobs() ([]cron.Job, error) {
	output, err := b.client.Run("crontab -l 2>/dev/null || true")
	if err != nil {
		return nil, fmt.Errorf("read crontab: %w", err)
	}
	return cron.Parse(output), nil
}

func (b *RemoteBackend) WriteJobs(jobs []cron.Job) error {
	content := cron.FormatCrontab(jobs)
	_, err := b.client.Run(fmt.Sprintf("echo %s | crontab -",
		shellQuote(content)))
	if err != nil {
		return fmt.Errorf("write crontab: %w", err)
	}
	return nil
}

func (b *RemoteBackend) RunJob(command string) (string, error) {
	output, err := b.client.Run(fmt.Sprintf("sh -c %s", shellQuote(command)))
	return output, err
}

func (b *RemoteBackend) LoadHistory() ([]history.Entry, error) {
	files, err := b.client.ListFiles("~/.lazycron/history", "*.json")
	if err != nil {
		return nil, fmt.Errorf("list history: %w", err)
	}

	// Limit to most recent 200 files
	if len(files) > 200 {
		// Files from ls are sorted alphabetically (timestamp-prefixed = chronological)
		files = files[len(files)-200:]
	}

	var entries []history.Entry
	for _, f := range files {
		data, err := b.client.ReadFile(f)
		if err != nil {
			continue
		}
		var e history.Entry
		if err := json.Unmarshal(data, &e); err != nil {
			continue
		}
		e.FilePath = f
		entries = append(entries, e)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp > entries[j].Timestamp
	})

	return entries, nil
}

func (b *RemoteBackend) WriteHistory(jobName, output string, success bool) error {
	now := time.Now()
	e := record.Entry{
		JobName:   jobName,
		Timestamp: now.Format(time.RFC3339),
		Output:    output,
		Success:   &success,
	}

	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return err
	}

	safeName := strings.ReplaceAll(jobName, "/", "_")
	safeName = strings.ReplaceAll(safeName, " ", "_")
	filename := now.Format("2006-01-02T15-04-05") + "_" + safeName + ".json"
	path := "~/.lazycron/history/" + filename

	return b.client.Upload(string(data), path, 0o644)
}

func (b *RemoteBackend) EnsureRecordScript() error {
	if err := b.client.Connect(); err != nil {
		return err
	}

	// Create directories
	_, err := b.client.Run("mkdir -p ~/.lazycron/bin ~/.lazycron/history")
	if err != nil {
		return fmt.Errorf("create dirs: %w", err)
	}

	// Upload record script
	return b.client.Upload(string(record.ScriptContent), "~/.lazycron/bin/record", 0o755)
}

func (b *RemoteBackend) Close() error {
	return b.client.Close()
}

// shellQuote wraps a string in single quotes with proper escaping.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
