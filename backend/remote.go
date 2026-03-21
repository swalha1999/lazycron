package backend

import (
	"encoding/json"
	"fmt"
	"path/filepath"
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

// SetPassword sets the password on the underlying SSH client for runtime auth.
func (b *RemoteBackend) SetPassword(pw string) {
	b.client.SetPassword(pw)
}

func (b *RemoteBackend) ReadJobs() ([]cron.Job, error) {
	output, err := b.client.Run("crontab -l 2>/dev/null || true")
	if err != nil {
		return nil, fmt.Errorf("read crontab: %w", err)
	}
	jobs := cron.Parse(output)

	// Resolve script refs that couldn't be resolved locally (remote files).
	for i, j := range jobs {
		if cron.IsScriptRef(j.Command) {
			path := strings.TrimPrefix(j.Command, "sh ")
			data, readErr := b.client.ReadFile(path)
			if readErr == nil {
				content := string(data)
				if strings.HasPrefix(content, "#!/bin/sh\n") {
					content = content[len("#!/bin/sh\n"):]
				}
				jobs[i].Command = strings.TrimRight(content, "\n")
			}
		}
	}

	return jobs, nil
}

func (b *RemoteBackend) WriteJobs(jobs []cron.Job) error {
	// Get the remote home directory for correct script paths.
	remoteHome, err := b.client.Run("echo $HOME")
	if err != nil {
		return fmt.Errorf("get remote home: %w", err)
	}
	remoteHome = strings.TrimSpace(remoteHome)
	remoteScriptsDir := remoteHome + "/.lazycron/scripts"

	// Ensure scripts directory exists on remote.
	if _, err := b.client.Run("mkdir -p " + remoteScriptsDir); err != nil {
		return fmt.Errorf("create scripts dir: %w", err)
	}

	// Upload script files for each job.
	active := make(map[string]bool)
	for _, j := range jobs {
		filename := filepath.Base(cron.ScriptPath(j.Name))
		active[filename] = true
		content := "#!/bin/sh\n" + j.Command + "\n"
		path := remoteScriptsDir + "/" + filename
		if err := b.client.Upload(content, path, 0o755); err != nil {
			return fmt.Errorf("upload script %s: %w", j.Name, err)
		}
	}

	// Delete orphan scripts on remote.
	remoteFiles, _ := b.client.ListFiles(remoteScriptsDir, "*.sh")
	for _, f := range remoteFiles {
		base := filepath.Base(f)
		if !active[base] {
			b.client.Run("rm -f " + shellQuote(f))
		}
	}

	// Format crontab, replacing local script paths with remote paths.
	crontabContent := cron.FormatCrontab(jobs)
	crontabContent = strings.ReplaceAll(crontabContent, cron.ScriptsDir(), remoteScriptsDir)

	_, err = b.client.Run(fmt.Sprintf("echo %s | crontab -",
		shellQuote(crontabContent)))
	if err != nil {
		return fmt.Errorf("write crontab: %w", err)
	}
	return nil
}

func (b *RemoteBackend) RunJob(name, command string) (string, error) {
	// Get remote home for script path.
	remoteHome, err := b.client.Run("echo $HOME")
	if err != nil {
		return "", fmt.Errorf("get remote home: %w", err)
	}
	remoteHome = strings.TrimSpace(remoteHome)
	scriptPath := remoteHome + "/.lazycron/scripts/" + filepath.Base(cron.ScriptPath(name))

	// Upload script to remote.
	content := "#!/bin/sh\n" + command + "\n"
	if err := b.client.Upload(content, scriptPath, 0o755); err != nil {
		return "", fmt.Errorf("upload script: %w", err)
	}

	return b.client.Run("sh " + shellQuote(scriptPath))
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
	_, err := b.client.Run("mkdir -p ~/.lazycron/bin ~/.lazycron/history ~/.lazycron/scripts")
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
