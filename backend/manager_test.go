package backend

import (
	"testing"
	"time"

	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/history"
)

// mockBackend implements Backend for testing.
type mockBackend struct {
	name   string
	closed bool
}

func (m *mockBackend) Name() string                                        { return m.name }
func (m *mockBackend) ReadJobs() ([]cron.Job, error)                       { return nil, nil }
func (m *mockBackend) WriteJobs(jobs []cron.Job) error                     { return nil }
func (m *mockBackend) RunJob(id, name, command string) (string, error)           { return "", nil }
func (m *mockBackend) LoadHistory() ([]history.Entry, error)                     { return nil, nil }
func (m *mockBackend) WriteHistory(jobID, jobName, output string, ok bool) error { return nil }
func (m *mockBackend) DeleteHistory(filePath string) error                 { return nil }
func (m *mockBackend) EnsureRecordScript() error                           { return nil }
func (m *mockBackend) GetTimezone() (string, int, error)                  { return "UTC", 0, nil }
func (m *mockBackend) Close() error                                        { m.closed = true; return nil }

// newTestManager creates a Manager with a mock local backend (avoids system crontab).
func newTestManager() *Manager {
	local := &mockBackend{name: "local"}
	return &Manager{
		backends: []Backend{local},
		servers:  []ServerInfo{{Name: "local", Status: ConnLocal}},
		active:   0,
		cache:    make(map[int]*CachedData),
	}
}

// --- NewManager ---

func TestNewManager_HasLocalBackend(t *testing.T) {
	m := newTestManager()
	if m.ServerCount() != 1 {
		t.Fatalf("expected 1 server, got %d", m.ServerCount())
	}
	if m.ActiveIndex() != 0 {
		t.Errorf("active index = %d, want 0", m.ActiveIndex())
	}
	if m.ActiveBackend().Name() != "local" {
		t.Errorf("active backend name = %q, want %q", m.ActiveBackend().Name(), "local")
	}
}

// --- AddServer ---

func TestAddServer(t *testing.T) {
	m := newTestManager()
	b := &mockBackend{name: "prod"}
	m.AddServer(ServerInfo{Name: "prod", Host: "prod.example.com", Status: ConnDisconnected}, b)

	if m.ServerCount() != 2 {
		t.Fatalf("expected 2 servers, got %d", m.ServerCount())
	}
	if m.BackendAt(1).Name() != "prod" {
		t.Errorf("backend[1] name = %q, want %q", m.BackendAt(1).Name(), "prod")
	}
}

// --- SwitchTo ---

func TestSwitchTo(t *testing.T) {
	m := newTestManager()
	m.AddServer(ServerInfo{Name: "staging"}, &mockBackend{name: "staging"})

	m.SwitchTo(1)
	if m.ActiveIndex() != 1 {
		t.Errorf("active = %d, want 1", m.ActiveIndex())
	}
	if m.ActiveBackend().Name() != "staging" {
		t.Errorf("active backend = %q, want %q", m.ActiveBackend().Name(), "staging")
	}

	m.SwitchTo(0)
	if m.ActiveIndex() != 0 {
		t.Errorf("active = %d after switch back, want 0", m.ActiveIndex())
	}
}

func TestSwitchTo_OutOfBounds(t *testing.T) {
	m := newTestManager()

	m.SwitchTo(-1)
	if m.ActiveIndex() != 0 {
		t.Errorf("negative index should be ignored, active = %d", m.ActiveIndex())
	}

	m.SwitchTo(99)
	if m.ActiveIndex() != 0 {
		t.Errorf("out-of-bounds index should be ignored, active = %d", m.ActiveIndex())
	}
}

// --- Servers / ServerAt / BackendAt ---

func TestServers_ReturnsCopy(t *testing.T) {
	m := newTestManager()
	servers := m.Servers()
	servers[0].Name = "modified"

	// Original should not be affected
	if m.ServerAt(0).Name != "local" {
		t.Error("Servers() should return a copy, not a reference")
	}
}

func TestServerAt_OutOfBounds(t *testing.T) {
	m := newTestManager()
	s := m.ServerAt(-1)
	if s.Name != "" {
		t.Errorf("ServerAt(-1) should return zero value, got %+v", s)
	}
	s = m.ServerAt(99)
	if s.Name != "" {
		t.Errorf("ServerAt(99) should return zero value, got %+v", s)
	}
}

func TestBackendAt_OutOfBounds(t *testing.T) {
	m := newTestManager()
	if m.BackendAt(-1) != nil {
		t.Error("BackendAt(-1) should return nil")
	}
	if m.BackendAt(99) != nil {
		t.Error("BackendAt(99) should return nil")
	}
}

// --- SetServerStatus ---

func TestSetServerStatus(t *testing.T) {
	m := newTestManager()
	m.AddServer(ServerInfo{Name: "prod", Status: ConnDisconnected}, &mockBackend{name: "prod"})

	m.SetServerStatus(1, ConnConnected, "")
	s := m.ServerAt(1)
	if s.Status != ConnConnected {
		t.Errorf("status = %d, want ConnConnected (%d)", s.Status, ConnConnected)
	}

	m.SetServerStatus(1, ConnError, "connection refused")
	s = m.ServerAt(1)
	if s.Status != ConnError || s.Error != "connection refused" {
		t.Errorf("status = %d, error = %q; want ConnError with message", s.Status, s.Error)
	}
}

func TestSetServerStatus_OutOfBounds(t *testing.T) {
	m := newTestManager()
	// Should not panic
	m.SetServerStatus(-1, ConnError, "bad")
	m.SetServerStatus(99, ConnError, "bad")
}

// --- Cache ---

func TestCache_SetAndGet(t *testing.T) {
	m := newTestManager()

	if m.GetCache(0) != nil {
		t.Error("cache should be nil initially")
	}

	data := &CachedData{
		Jobs:      []cron.Job{{Name: "test", Schedule: "* * * * *"}},
		FetchedAt: time.Now(),
	}
	m.SetCache(0, data)

	got := m.GetCache(0)
	if got == nil {
		t.Fatal("cache should not be nil after SetCache")
	}
	if len(got.Jobs) != 1 || got.Jobs[0].Name != "test" {
		t.Errorf("cached jobs mismatch: %+v", got.Jobs)
	}
}

func TestCache_Invalidate(t *testing.T) {
	m := newTestManager()
	m.SetCache(0, &CachedData{FetchedAt: time.Now()})

	m.InvalidateCache(0)
	if m.GetCache(0) != nil {
		t.Error("cache should be nil after InvalidateCache")
	}
}

func TestIsCacheFresh(t *testing.T) {
	m := newTestManager()

	// No cache → not fresh
	if m.IsCacheFresh(0) {
		t.Error("no cache should not be fresh")
	}

	// Fresh cache
	m.SetCache(0, &CachedData{FetchedAt: time.Now()})
	if !m.IsCacheFresh(0) {
		t.Error("just-set cache should be fresh")
	}

	// Stale cache
	m.SetCache(0, &CachedData{FetchedAt: time.Now().Add(-CacheFreshness - time.Second)})
	if m.IsCacheFresh(0) {
		t.Error("old cache should not be fresh")
	}
}

// --- RemoveServer ---

func TestRemoveServer(t *testing.T) {
	m := newTestManager()
	b1 := &mockBackend{name: "prod"}
	b2 := &mockBackend{name: "staging"}
	m.AddServer(ServerInfo{Name: "prod"}, b1)
	m.AddServer(ServerInfo{Name: "staging"}, b2)

	ok := m.RemoveServer(1) // remove "prod"
	if !ok {
		t.Fatal("RemoveServer should return true")
	}
	if !b1.closed {
		t.Error("removed backend should be closed")
	}
	if m.ServerCount() != 2 {
		t.Fatalf("expected 2 servers, got %d", m.ServerCount())
	}
	if m.ServerAt(1).Name != "staging" {
		t.Errorf("server[1] = %q, want %q", m.ServerAt(1).Name, "staging")
	}
}

func TestRemoveServer_CannotRemoveLocal(t *testing.T) {
	m := newTestManager()
	ok := m.RemoveServer(0)
	if ok {
		t.Error("should not be able to remove local (index 0)")
	}
	if m.ServerCount() != 1 {
		t.Errorf("server count changed after failed remove: %d", m.ServerCount())
	}
}

func TestRemoveServer_OutOfBounds(t *testing.T) {
	m := newTestManager()
	if m.RemoveServer(-1) {
		t.Error("negative index should return false")
	}
	if m.RemoveServer(99) {
		t.Error("out-of-bounds index should return false")
	}
}

func TestRemoveServer_ActiveWasRemoved(t *testing.T) {
	m := newTestManager()
	m.AddServer(ServerInfo{Name: "prod"}, &mockBackend{name: "prod"})

	m.SwitchTo(1)
	if m.ActiveIndex() != 1 {
		t.Fatalf("active should be 1, got %d", m.ActiveIndex())
	}

	m.RemoveServer(1)
	if m.ActiveIndex() != 0 {
		t.Errorf("active should reset to 0 after removing active server, got %d", m.ActiveIndex())
	}
}

func TestRemoveServer_ActiveAfterLowerRemoved(t *testing.T) {
	m := newTestManager()
	m.AddServer(ServerInfo{Name: "prod"}, &mockBackend{name: "prod"})
	m.AddServer(ServerInfo{Name: "staging"}, &mockBackend{name: "staging"})

	m.SwitchTo(2) // active = staging (index 2)
	m.RemoveServer(1) // remove prod (index 1)

	// Active was 2, removed index 1, so active should shift to 1
	if m.ActiveIndex() != 1 {
		t.Errorf("active should shift down to 1, got %d", m.ActiveIndex())
	}
	if m.ActiveBackend().Name() != "staging" {
		t.Errorf("active backend should still be staging, got %q", m.ActiveBackend().Name())
	}
}

func TestRemoveServer_InvalidatesCache(t *testing.T) {
	m := newTestManager()
	m.AddServer(ServerInfo{Name: "prod"}, &mockBackend{name: "prod"})
	m.SetCache(1, &CachedData{FetchedAt: time.Now()})

	m.RemoveServer(1)
	if m.GetCache(1) != nil {
		t.Error("cache for removed server should be invalidated")
	}
}

// --- CloseAll ---

func TestCloseAll(t *testing.T) {
	local := &mockBackend{name: "local"}
	m := &Manager{
		backends: []Backend{local},
		servers:  []ServerInfo{{Name: "local", Status: ConnLocal}},
		cache:    make(map[int]*CachedData),
	}

	remote1 := &mockBackend{name: "prod"}
	remote2 := &mockBackend{name: "staging"}
	m.AddServer(ServerInfo{Name: "prod"}, remote1)
	m.AddServer(ServerInfo{Name: "staging"}, remote2)

	m.CloseAll()

	if !local.closed {
		t.Error("local backend should be closed")
	}
	if !remote1.closed {
		t.Error("remote1 backend should be closed")
	}
	if !remote2.closed {
		t.Error("remote2 backend should be closed")
	}
}

// --- shellQuote ---

func TestShellQuote(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "'hello'"},
		{"hello world", "'hello world'"},
		{"it's", "'it'\\''s'"},
		{"", "''"},
		{`"quoted"`, `'"quoted"'`},
		{"line1\nline2", "'line1\nline2'"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := shellQuote(tt.input)
			if got != tt.want {
				t.Errorf("shellQuote(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
