package backend

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/history"
	"github.com/swalha1999/lazycron/record"
	sshclient "github.com/swalha1999/lazycron/ssh"
)

// RemoteBackend implements Backend over an SSH connection.
type RemoteBackend struct {
	name       string
	client     *sshclient.Client
	remoteHome string // cached remote $HOME (set during EnsureRecordScript)
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

// getRemoteHome returns the cached remote home directory, fetching it if needed.
func (b *RemoteBackend) getRemoteHome() (string, error) {
	if b.remoteHome != "" {
		return b.remoteHome, nil
	}
	home, err := b.client.Run("echo $HOME")
	if err != nil {
		return "", fmt.Errorf("get remote home: %w", err)
	}
	b.remoteHome = strings.TrimSpace(home)
	return b.remoteHome, nil
}

// lazycronDir returns the remote ~/.lazycron path using absolute paths.
func (b *RemoteBackend) lazycronDir() (string, error) {
	home, err := b.getRemoteHome()
	if err != nil {
		return "", err
	}
	return home + "/.lazycron", nil
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
			path := strings.Trim(strings.TrimPrefix(j.Command, "sh "), "'\"")
			data, readErr := b.client.ReadFile(path)
			if readErr == nil {
				jobs[i].Command = cron.StripShebang(string(data))
			}
		}
	}

	return jobs, nil
}

func (b *RemoteBackend) WriteJobs(jobs []cron.Job) error {
	lcDir, err := b.lazycronDir()
	if err != nil {
		return err
	}
	remoteHome := b.remoteHome
	remoteScriptsDir := lcDir + "/scripts"

	// Ensure scripts directory exists on remote.
	if _, err := b.client.Run("mkdir -p " + remoteScriptsDir); err != nil {
		return fmt.Errorf("create scripts dir: %w", err)
	}

	// Upload script files for each job.
	active := make(map[string]bool)
	for _, j := range jobs {
		filename := filepath.Base(cron.ScriptPath(j.ID))
		active[filename] = true
		content := cron.BuildScriptContent(j.Command)
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

	// Format crontab, replacing local paths with remote paths.
	crontabContent := cron.FormatCrontab(jobs)
	crontabContent = strings.ReplaceAll(crontabContent, cron.ScriptsDir(), remoteScriptsDir)
	remoteRecordBin := remoteHome + "/.lazycron/bin/record"
	crontabContent = strings.ReplaceAll(crontabContent, cron.RecordBinPath(), remoteRecordBin)

	_, err = b.client.Run(fmt.Sprintf("echo %s | crontab -",
		shellQuote(crontabContent)))
	if err != nil {
		return fmt.Errorf("write crontab: %w", err)
	}
	return nil
}

func (b *RemoteBackend) RunJob(id, name, command string) (string, error) {
	lcDir, err := b.lazycronDir()
	if err != nil {
		return "", err
	}
	scriptPath := lcDir + "/scripts/" + filepath.Base(cron.ScriptPath(id))

	// Upload script to remote.
	if err := b.client.Upload(cron.BuildScriptContent(command), scriptPath, 0o755); err != nil {
		return "", fmt.Errorf("upload script: %w", err)
	}

	return b.client.Run("sh " + shellQuote(scriptPath))
}

func (b *RemoteBackend) LoadHistory() ([]history.Entry, error) {
	lcDir, err := b.lazycronDir()
	if err != nil {
		return nil, err
	}
	files, err := b.client.ListFiles(lcDir+"/history", "*.json")
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

func (b *RemoteBackend) WriteHistory(jobID, jobName, output string, success bool) error {
	filename, data, err := history.BuildHistoryFile(jobID, jobName, output, success)
	if err != nil {
		return err
	}

	lcDir, lcErr := b.lazycronDir()
	if lcErr != nil {
		return lcErr
	}

	return b.client.Upload(string(data), lcDir+"/history/"+filename, 0o644)
}

func (b *RemoteBackend) DeleteHistory(filePath string) error {
	_, err := b.client.Run("rm -f " + shellQuote(filePath))
	return err
}

func (b *RemoteBackend) EnsureRecordScript() error {
	if err := b.client.Connect(); err != nil {
		return err
	}

	lcDir, err := b.lazycronDir()
	if err != nil {
		return err
	}

	// Create directories
	_, err = b.client.Run(fmt.Sprintf("mkdir -p %s/bin %s/history %s/scripts",
		lcDir, lcDir, lcDir))
	if err != nil {
		return fmt.Errorf("create dirs: %w", err)
	}

	// Upload record script
	return b.client.Upload(string(record.ScriptContent), lcDir+"/bin/record", 0o755)
}

// DirLister returns a RemoteDirLister for path completion on this server.
func (b *RemoteBackend) DirLister() *RemoteDirLister {
	return NewRemoteDirLister(b.client)
}

func (b *RemoteBackend) Close() error {
	return b.client.Close()
}

// shellQuote is a package-level alias for cron.ShellQuote.
var shellQuote = cron.ShellQuote
