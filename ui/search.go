package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/swalha1999/lazycron/backend"
	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/history"
)

func newSearchInput() textinput.Model {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "type to filter..."
	ti.CharLimit = 128
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(colorMuted)
	ti.TextStyle = lipgloss.NewStyle().Foreground(colorFg)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(colorHighlight)
	return ti
}

// enterSearch switches to search mode for the currently focused panel.
func (m *Model) enterSearch() tea.Cmd {
	panel := m.focusPanel
	if panel == panelDetail {
		panel = m.lastLeftPanel
	}
	m.searchInput = newSearchInput()
	if m.searchPanel == panel && m.searchQuery != "" {
		m.searchInput.SetValue(m.searchQuery)
	}
	m.searchPanel = panel
	m.mode = modeSearch
	m.statusMsg = ""
	return m.searchInput.Focus()
}

// clearSearch removes the active search filter and restores full lists.
func (m *Model) clearSearch() {
	m.searchQuery = ""
	m.searchPanel = -1
	m.searchJobMatch = nil
}

// hasActiveSearch returns true if a search filter is currently applied.
func (m *Model) hasActiveSearch() bool {
	return m.searchPanel >= 0 && m.searchQuery != ""
}

// applySearch recomputes filtered data based on the current search query.
func (m *Model) applySearch(query string) {
	m.searchQuery = query
	if query == "" {
		m.searchJobMatch = nil
		return
	}

	switch m.searchPanel {
	case panelJobs:
		m.searchJobMatch = matchJobs(m.jobs, query)
		m.clampSelectedRow()
	case panelServers:
		// Clamp server selection to visible items
		servers := m.manager.Servers()
		visible := matchServers(servers, query)
		if len(visible) > 0 && !visible[m.serverSelected] {
			for i := 0; i < len(servers); i++ {
				if visible[i] {
					m.serverSelected = i
					break
				}
			}
		}
	case panelHistory:
		visible := matchHistory(m.history, query)
		if len(visible) > 0 && !visible[m.historySelected] {
			for i := 0; i < len(m.history); i++ {
				if visible[i] {
					m.historySelected = i
					break
				}
			}
		}
	}
}

// matchJobs returns a set of job indices that match the query.
func matchJobs(jobs []cron.Job, query string) map[int]bool {
	matched := make(map[int]bool)
	for i, job := range jobs {
		searchText := strings.ToLower(job.Name + " " + job.Tag + " " + cron.CronToHuman(job.Schedule) + " " + job.Project)
		if fuzzyMatchSimple(searchText, strings.ToLower(query)) {
			matched[i] = true
		}
	}
	return matched
}

// matchServers returns a set of server indices that match the query.
func matchServers(servers []backend.ServerInfo, query string) map[int]bool {
	matched := make(map[int]bool)
	for i, srv := range servers {
		searchText := strings.ToLower(srv.Name + " " + srv.Host)
		if fuzzyMatchSimple(searchText, strings.ToLower(query)) {
			matched[i] = true
		}
	}
	return matched
}

// matchHistory returns a set of history indices that match the query.
func matchHistory(entries []history.Entry, query string) map[int]bool {
	matched := make(map[int]bool)
	for i, entry := range entries {
		searchText := strings.ToLower(entry.JobName + " " + entry.Timestamp)
		if fuzzyMatchSimple(searchText, strings.ToLower(query)) {
			matched[i] = true
		}
	}
	return matched
}

// fuzzyMatchSimple does a case-insensitive subsequence match.
// Both text and pattern should already be lowercased.
func fuzzyMatchSimple(text, pattern string) bool {
	pi := 0
	for ni := 0; ni < len(text) && pi < len(pattern); ni++ {
		if text[ni] == pattern[pi] {
			pi++
		}
	}
	return pi >= len(pattern)
}

// serverMatchSet returns the current match set for servers, or nil if no search is active.
func (m *Model) serverMatchSet() map[int]bool {
	if m.searchPanel != panelServers || m.searchQuery == "" {
		return nil
	}
	return matchServers(m.manager.Servers(), m.searchQuery)
}

// historyMatchSet returns the current match set for history, or nil if no search is active.
func (m *Model) historyMatchSet() map[int]bool {
	if m.searchPanel != panelHistory || m.searchQuery == "" {
		return nil
	}
	return matchHistory(m.history, m.searchQuery)
}

// nextVisibleServer finds the next visible server index in the given direction.
func (m *Model) nextVisibleServer(from, direction int) int {
	matchSet := m.serverMatchSet()
	if matchSet == nil {
		return from
	}
	count := m.manager.ServerCount()
	for i := from; i >= 0 && i < count; i += direction {
		if matchSet[i] {
			return i
		}
	}
	return -1
}

// nextVisibleHistory finds the next visible history index in the given direction.
func (m *Model) nextVisibleHistory(from, direction int) int {
	matchSet := m.historyMatchSet()
	if matchSet == nil {
		return from
	}
	for i := from; i >= 0 && i < len(m.history); i += direction {
		if matchSet[i] {
			return i
		}
	}
	return -1
}

// visibleServerCount returns (matched, total) counts for servers.
func (m *Model) visibleServerCount() (int, int) {
	total := m.manager.ServerCount()
	matchSet := m.serverMatchSet()
	if matchSet == nil {
		return total, total
	}
	return len(matchSet), total
}

// visibleHistoryCount returns (matched, total) counts for history.
func (m *Model) visibleHistoryCount() (int, int) {
	total := len(m.history)
	matchSet := m.historyMatchSet()
	if matchSet == nil {
		return total, total
	}
	return len(matchSet), total
}

// visibleJobCount returns (matched, total) counts for jobs.
func (m *Model) visibleJobCount() (int, int) {
	total := len(m.jobs)
	if m.searchJobMatch == nil {
		return total, total
	}
	return len(m.searchJobMatch), total
}

// handleSearchKey handles key events during search mode.
func (m Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		// Lock the filter and return to normal mode
		query := strings.TrimSpace(m.searchInput.Value())
		if query == "" {
			m.clearSearch()
		} else {
			m.searchQuery = query
			m.applySearch(query)
		}
		m.mode = modeNormal
		return m, nil
	case "esc":
		// Clear filter and return to normal mode
		m.clearSearch()
		m.mode = modeNormal
		return m, nil
	default:
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		// Apply filter in real-time
		m.applySearch(m.searchInput.Value())
		return m, cmd
	}
}
