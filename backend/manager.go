package backend

import (
	"sync"
	"time"

	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/history"
)

// ConnStatus represents a server's connection state.
type ConnStatus int

const (
	ConnLocal ConnStatus = iota
	ConnDisconnected
	ConnConnecting
	ConnConnected
	ConnError
)

// ServerInfo holds metadata about a configured server.
type ServerInfo struct {
	Name           string
	Host           string
	Port           int
	User           string
	Status         ConnStatus
	Error          string
	Timezone       string // e.g. "UTC", "EST", "PST"
	TimezoneOffset int    // offset in seconds from UTC
}

// CachedData holds cached jobs and history for a server.
type CachedData struct {
	Jobs      []cron.Job
	History   []history.Entry
	FetchedAt time.Time
}

// CacheFreshness is the duration before cached data is considered stale.
const CacheFreshness = 30 * time.Second

// Manager orchestrates multiple backends and caching.
type Manager struct {
	mu       sync.Mutex
	backends []Backend
	servers  []ServerInfo
	active   int
	cache    map[int]*CachedData
}

// NewManager creates a manager with a local backend always at index 0.
func NewManager() *Manager {
	local := NewLocalBackend()
	return &Manager{
		backends: []Backend{local},
		servers: []ServerInfo{
			{Name: "local", Status: ConnLocal},
		},
		active: 0,
		cache:  make(map[int]*CachedData),
	}
}

// NewManagerWithBackend creates a manager with a custom backend at index 0.
func NewManagerWithBackend(b Backend) *Manager {
	return &Manager{
		backends: []Backend{b},
		servers: []ServerInfo{
			{Name: b.Name(), Status: ConnLocal},
		},
		active: 0,
		cache:  make(map[int]*CachedData),
	}
}

// AddServer registers a remote server backend.
func (m *Manager) AddServer(info ServerInfo, b Backend) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.servers = append(m.servers, info)
	m.backends = append(m.backends, b)
}

// ActiveBackend returns the currently selected backend.
func (m *Manager) ActiveBackend() Backend {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.backends[m.active]
}

// ActiveIndex returns the index of the active server.
func (m *Manager) ActiveIndex() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.active
}

// SwitchTo changes the active server.
func (m *Manager) SwitchTo(index int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if index >= 0 && index < len(m.backends) {
		m.active = index
	}
}

// ServerCount returns the number of registered servers.
func (m *Manager) ServerCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.servers)
}

// Servers returns a copy of server info.
func (m *Manager) Servers() []ServerInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]ServerInfo, len(m.servers))
	copy(out, m.servers)
	return out
}

// ServerAt returns the info for a specific server index.
func (m *Manager) ServerAt(index int) ServerInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	if index >= 0 && index < len(m.servers) {
		return m.servers[index]
	}
	return ServerInfo{}
}

// BackendAt returns the backend for a specific server index.
func (m *Manager) BackendAt(index int) Backend {
	m.mu.Lock()
	defer m.mu.Unlock()
	if index >= 0 && index < len(m.backends) {
		return m.backends[index]
	}
	return nil
}

// SetServerStatus updates a server's connection status.
func (m *Manager) SetServerStatus(index int, status ConnStatus, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if index >= 0 && index < len(m.servers) {
		m.servers[index].Status = status
		m.servers[index].Error = errMsg
	}
}

// SetServerTimezone updates a server's timezone information.
func (m *Manager) SetServerTimezone(index int, timezone string, offset int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if index >= 0 && index < len(m.servers) {
		m.servers[index].Timezone = timezone
		m.servers[index].TimezoneOffset = offset
	}
}

// GetCache returns cached data for a server (nil if no cache).
func (m *Manager) GetCache(index int) *CachedData {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cache[index]
}

// SetCache stores cached data for a server.
func (m *Manager) SetCache(index int, data *CachedData) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cache[index] = data
}

// InvalidateCache removes cached data for a server.
func (m *Manager) InvalidateCache(index int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.cache, index)
}

// IsCacheFresh returns whether the cache for a server is still valid.
func (m *Manager) IsCacheFresh(index int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.cache[index]
	if !ok {
		return false
	}
	return time.Since(c.FetchedAt) < CacheFreshness
}

// RemoveServer removes a server by index (cannot remove local at index 0).
// If the removed server was active, switches to local.
func (m *Manager) RemoveServer(index int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if index <= 0 || index >= len(m.servers) {
		return false
	}
	// Close the backend being removed
	m.backends[index].Close()
	m.servers = append(m.servers[:index], m.servers[index+1:]...)
	m.backends = append(m.backends[:index], m.backends[index+1:]...)
	delete(m.cache, index)
	// Fix active index
	if m.active == index {
		m.active = 0
	} else if m.active > index {
		m.active--
	}
	return true
}

// CloseAll closes all backends.
func (m *Manager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, b := range m.backends {
		b.Close()
	}
}
