package notify

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHasNotifications(t *testing.T) {
	empty := Config{}
	if empty.HasNotifications() {
		t.Error("empty config should not have notifications")
	}

	withFailure := Config{
		OnFailure: []Action{{Type: "webhook", URL: "https://example.com"}},
	}
	if !withFailure.HasNotifications() {
		t.Error("config with on_failure should have notifications")
	}

	withSuccess := Config{
		OnSuccess: []Action{{Type: "command", Run: "echo done"}},
	}
	if !withSuccess.HasNotifications() {
		t.Error("config with on_success should have notifications")
	}
}

func TestWriteJobConfig_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	origDir := NotifyDir
	NotifyDir = func() string { return dir }
	defer func() { NotifyDir = origDir }()

	cfg := Config{
		OnFailure: []Action{
			{Type: "webhook", URL: "https://hooks.slack.com/test"},
			{Type: "command", Run: "notify-send 'lazycron' '{{.JobName}} failed'"},
		},
		OnSuccess: []Action{
			{Type: "desktop"},
		},
	}

	if err := WriteJobConfig("test-job", "0 3 * * *", cfg); err != nil {
		t.Fatalf("WriteJobConfig: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "test-job.conf"))
	if err != nil {
		t.Fatalf("read config file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "meta\tschedule\t0 3 * * *") {
		t.Error("missing schedule metadata")
	}
	if !strings.Contains(content, "on_failure\twebhook\thttps://hooks.slack.com/test") {
		t.Error("missing webhook action")
	}
	if !strings.Contains(content, "on_failure\tcommand\tnotify-send") {
		t.Error("missing command action")
	}
	if !strings.Contains(content, "on_success\tdesktop") {
		t.Error("missing desktop action")
	}
}

func TestWriteJobConfig_RemovesFileWhenEmpty(t *testing.T) {
	dir := t.TempDir()
	origDir := NotifyDir
	NotifyDir = func() string { return dir }
	defer func() { NotifyDir = origDir }()

	// Write a config first.
	cfg := Config{
		OnFailure: []Action{{Type: "webhook", URL: "https://example.com"}},
	}
	if err := WriteJobConfig("test-job", "* * * * *", cfg); err != nil {
		t.Fatalf("WriteJobConfig: %v", err)
	}

	// Now write empty config — should remove the file.
	if err := WriteJobConfig("test-job", "* * * * *", Config{}); err != nil {
		t.Fatalf("WriteJobConfig empty: %v", err)
	}

	if HasJobConfig("test-job") {
		t.Error("expected config file to be removed for empty config")
	}
}

func TestHasJobConfig(t *testing.T) {
	dir := t.TempDir()
	origDir := NotifyDir
	NotifyDir = func() string { return dir }
	defer func() { NotifyDir = origDir }()

	if HasJobConfig("nonexistent") {
		t.Error("should not find config for nonexistent job")
	}

	os.WriteFile(filepath.Join(dir, "my-job.conf"), []byte("test"), 0o600)
	if !HasJobConfig("my-job") {
		t.Error("should find config for existing job")
	}
}

func TestSendWebhook(t *testing.T) {
	var received WebhookPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json, got %s", r.Header.Get("Content-Type"))
		}
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)
		w.WriteHeader(200)
	}))
	defer server.Close()

	data := TemplateData{
		JobName:   "Test Job",
		Schedule:  "0 3 * * *",
		ExitCode:  1,
		Output:    "error: connection refused",
		Server:    "production",
		Timestamp: "2026-03-28T03:00:12Z",
	}

	if err := sendWebhook(server.URL, data); err != nil {
		t.Fatalf("sendWebhook: %v", err)
	}

	if received.JobName != "Test Job" {
		t.Errorf("job_name = %q, want %q", received.JobName, "Test Job")
	}
	if received.ExitCode != 1 {
		t.Errorf("exit_code = %d, want 1", received.ExitCode)
	}
	if received.Output != "error: connection refused" {
		t.Errorf("output = %q", received.Output)
	}
	if received.Server != "production" {
		t.Errorf("server = %q", received.Server)
	}
}

func TestSendWebhook_TruncatesOutput(t *testing.T) {
	var received WebhookPayload
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)
		w.WriteHeader(200)
	}))
	defer server.Close()

	longOutput := strings.Repeat("x", 2000)
	data := TemplateData{
		JobName:  "Test",
		Output:   longOutput,
		ExitCode: 1,
	}

	if err := sendWebhook(server.URL, data); err != nil {
		t.Fatalf("sendWebhook: %v", err)
	}

	if len(received.Output) > 1000 {
		t.Errorf("output not truncated: len=%d", len(received.Output))
	}
}

func TestSend_OnFailure(t *testing.T) {
	var called bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(200)
	}))
	defer server.Close()

	cfg := Config{
		OnFailure: []Action{{Type: "webhook", URL: server.URL}},
	}

	data := TemplateData{ExitCode: 1, JobName: "Test"}
	if err := Send(cfg, data); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !called {
		t.Error("on_failure webhook should have been called")
	}
}

func TestSend_OnFailureNotTriggeredOnSuccess(t *testing.T) {
	var called bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(200)
	}))
	defer server.Close()

	cfg := Config{
		OnFailure: []Action{{Type: "webhook", URL: server.URL}},
	}

	data := TemplateData{ExitCode: 0, JobName: "Test"}
	if err := Send(cfg, data); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if called {
		t.Error("on_failure webhook should not fire on success")
	}
}

func TestSend_OnSuccess(t *testing.T) {
	var called bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(200)
	}))
	defer server.Close()

	cfg := Config{
		OnSuccess: []Action{{Type: "webhook", URL: server.URL}},
	}

	data := TemplateData{ExitCode: 0, JobName: "Test"}
	if err := Send(cfg, data); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !called {
		t.Error("on_success webhook should have been called")
	}
}

func TestRenderTemplate(t *testing.T) {
	data := TemplateData{
		JobName:   "DB Backup",
		ExitCode:  1,
		Server:    "prod",
		Schedule:  "0 3 * * *",
		Timestamp: "2026-03-28T03:00:12Z",
	}

	result, err := renderTemplate("{{.JobName}} failed with exit {{.ExitCode}} on {{.Server}}", data)
	if err != nil {
		t.Fatalf("renderTemplate: %v", err)
	}

	want := "DB Backup failed with exit 1 on prod"
	if result != want {
		t.Errorf("got %q, want %q", result, want)
	}
}
