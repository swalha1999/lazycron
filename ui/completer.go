package ui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
)

const visibleRows = 8 // how many rows to show at once

type suggestion struct {
	name        string     // directory basename
	fullPath    string     // absolute path with trailing /
	matchRanges [][2]int   // matched char ranges in name for highlighting
	hasChildren bool       // has subdirectories
}

type completerModel struct {
	suggestions []suggestion
	selected    int
	active      bool
	browseDir   string // directory currently being listed (absolute, trailing /)
	filterText  string // text typed after the last /
	noMatches   bool   // true when filter has no results
	permError   bool   // true when directory is unreadable
	emptyDir    bool   // true when directory has no subdirs
}

// activate initializes the completer for a given input value.
func (c *completerModel) activate(input string) {
	input = strings.TrimSpace(input)
	if input == "" {
		c.showSeeds()
		return
	}
	c.parseInput(input)
	c.refresh()
}

// update refreshes suggestions as the user types.
func (c *completerModel) update(input string) {
	input = strings.TrimSpace(input)
	if input == "" {
		c.showSeeds()
		return
	}
	c.parseInput(input)
	c.refresh()
}

// parseInput splits input into browseDir and filterText.
func (c *completerModel) parseInput(input string) {
	expanded := expandTilde(input)
	if strings.HasSuffix(expanded, "/") {
		c.browseDir = expanded
		c.filterText = ""
	} else {
		c.browseDir = filepath.Dir(expanded) + "/"
		c.filterText = filepath.Base(expanded)
	}
}

// showSeeds shows common starting directories when input is empty.
func (c *completerModel) showSeeds() {
	var seeds []suggestion

	if home, err := os.UserHomeDir(); err == nil {
		seeds = append(seeds, suggestion{
			name:        "~",
			fullPath:    home + "/",
			hasChildren: true,
		})
	}

	if cwd, err := os.Getwd(); err == nil {
		seeds = append(seeds, suggestion{
			name:        ".",
			fullPath:    cwd + "/",
			hasChildren: true,
		})
	}

	seeds = append(seeds, suggestion{
		name:        "/tmp",
		fullPath:    "/tmp/",
		hasChildren: true,
	})

	c.suggestions = seeds
	c.active = true
	c.selected = -1
	c.noMatches = false
	c.permError = false
	c.emptyDir = false
	c.browseDir = ""
	c.filterText = ""
}

// refresh lists and filters directory entries.
func (c *completerModel) refresh() {
	c.permError = false
	c.noMatches = false
	c.emptyDir = false

	entries, err := os.ReadDir(c.browseDir)
	if err != nil {
		c.suggestions = nil
		c.permError = true
		c.active = true
		c.selected = -1
		return
	}

	var dirs []dirEntry
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(c.filterText, ".") {
			continue
		}

		isDir := entry.IsDir()
		if !isDir && entry.Type()&os.ModeSymlink != 0 {
			target, err := filepath.EvalSymlinks(filepath.Join(c.browseDir, name))
			if err == nil {
				if info, err := os.Stat(target); err == nil {
					isDir = info.IsDir()
				}
			}
		}
		if !isDir {
			continue
		}
		dirs = append(dirs, dirEntry{name: name})
	}

	if len(dirs) == 0 {
		c.suggestions = nil
		c.emptyDir = true
		c.active = true
		c.selected = -1
		return
	}

	// Filter and score
	var scored []scoredEntry
	for _, d := range dirs {
		if c.filterText == "" {
			scored = append(scored, scoredEntry{name: d.name, score: 0})
		} else {
			score, ranges := fuzzyMatch(d.name, c.filterText)
			if score > 0 {
				scored = append(scored, scoredEntry{name: d.name, score: score, matchRanges: ranges})
			}
		}
	}

	if len(scored) == 0 {
		c.suggestions = nil
		c.noMatches = true
		c.active = true
		c.selected = -1
		return
	}

	// Sort by score descending, then alphabetically
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return scored[i].name < scored[j].name
	})

	// Build suggestions with hasChildren check
	c.suggestions = make([]suggestion, len(scored))
	for i, s := range scored {
		fullPath := filepath.Join(c.browseDir, s.name) + "/"
		c.suggestions[i] = suggestion{
			name:        s.name,
			fullPath:    fullPath,
			matchRanges: s.matchRanges,
			hasChildren: hasSubdirs(fullPath),
		}
	}

	c.active = true
	c.selected = -1
}

// drillIn enters the selected directory.
func (c *completerModel) drillIn() string {
	if c.selected < 0 || c.selected >= len(c.suggestions) {
		return ""
	}
	path := c.suggestions[c.selected].fullPath
	c.browseDir = path
	c.filterText = ""
	c.refresh()
	return path
}

// drillOut goes to parent directory.
func (c *completerModel) drillOut() string {
	if c.browseDir == "/" {
		return c.browseDir
	}
	parent := filepath.Dir(strings.TrimSuffix(c.browseDir, "/"))
	if !strings.HasSuffix(parent, "/") {
		parent += "/"
	}
	c.browseDir = parent
	c.filterText = ""
	c.refresh()
	return parent
}

func (c *completerModel) selectNext() {
	if len(c.suggestions) == 0 {
		return
	}
	c.selected++
	if c.selected >= len(c.suggestions) {
		c.selected = 0
	}
}

func (c *completerModel) selectPrev() {
	if len(c.suggestions) == 0 {
		return
	}
	if c.selected <= 0 {
		c.selected = len(c.suggestions) - 1
	} else {
		c.selected--
	}
}

func (c *completerModel) selectedValue() string {
	if c.selected >= 0 && c.selected < len(c.suggestions) {
		return c.suggestions[c.selected].fullPath
	}
	return ""
}

func (c *completerModel) reset() {
	c.suggestions = nil
	c.active = false
	c.selected = -1
	c.browseDir = ""
	c.filterText = ""
	c.noMatches = false
	c.permError = false
	c.emptyDir = false
}

// breadcrumb returns a display string like "~ / code / lazycron".
func (c *completerModel) breadcrumb(maxWidth int) string {
	if c.browseDir == "" {
		return "Select a directory"
	}

	display := collapseTilde(c.browseDir)
	display = strings.TrimSuffix(display, "/")

	parts := strings.Split(display, "/")
	// Remove empty parts
	var clean []string
	for _, p := range parts {
		if p != "" {
			clean = append(clean, p)
		}
	}

	result := strings.Join(clean, " / ")
	if len(result) > maxWidth-4 {
		// Truncate from the start
		for len(result) > maxWidth-8 && len(clean) > 2 {
			clean = clean[1:]
			result = "... / " + strings.Join(clean, " / ")
		}
	}

	return result
}

// --- Fuzzy matching ---

type dirEntry struct {
	name string
}

type scoredEntry struct {
	name        string
	score       int
	matchRanges [][2]int
}

// fuzzyMatch does a subsequence match of pattern against name.
// Returns score > 0 if matched, and the character ranges that matched.
func fuzzyMatch(name, pattern string) (int, [][2]int) {
	nameLower := strings.ToLower(name)
	patLower := strings.ToLower(pattern)

	// Find matched positions
	var positions []int
	pi := 0
	for ni := 0; ni < len(nameLower) && pi < len(patLower); ni++ {
		if nameLower[ni] == patLower[pi] {
			positions = append(positions, ni)
			pi++
		}
	}
	if pi < len(patLower) {
		return 0, nil // not a subsequence
	}

	// Score
	score := len(positions) // base: 1 per matched char

	for i, pos := range positions {
		// Bonus: match at start
		if pos == 0 {
			score += 10
		}
		// Bonus: word boundary
		if pos > 0 && isWordBoundary(rune(name[pos-1]), rune(name[pos])) {
			score += 5
		}
		// Bonus: consecutive match
		if i > 0 && positions[i-1] == pos-1 {
			score += 3
		}
	}

	// Bonus: exact prefix
	if strings.HasPrefix(nameLower, patLower) {
		score += 20
	}

	// Build ranges from positions (merge consecutive)
	var ranges [][2]int
	start := positions[0]
	end := positions[0] + 1
	for i := 1; i < len(positions); i++ {
		if positions[i] == end {
			end++
		} else {
			ranges = append(ranges, [2]int{start, end})
			start = positions[i]
			end = positions[i] + 1
		}
	}
	ranges = append(ranges, [2]int{start, end})

	return score, ranges
}

func isWordBoundary(prev, curr rune) bool {
	return prev == '_' || prev == '-' || prev == '.' ||
		(unicode.IsLower(prev) && unicode.IsUpper(curr))
}

// --- Helpers ---

func hasSubdirs(path string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() {
			return true
		}
		if e.Type()&os.ModeSymlink != 0 {
			target, err := filepath.EvalSymlinks(filepath.Join(path, e.Name()))
			if err == nil {
				if info, err := os.Stat(target); err == nil && info.IsDir() {
					return true
				}
			}
		}
	}
	return false
}

func expandTilde(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		if path == "~" {
			return home
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

func collapseTilde(path string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

// --- Rendering ---

func renderCompletions(c *completerModel, width int) string {
	if !c.active {
		return ""
	}

	dropWidth := width
	if dropWidth > 60 {
		dropWidth = 60
	}
	if dropWidth < 30 {
		dropWidth = 30
	}
	innerWidth := dropWidth - 4 // border + padding

	crumbStyle := lipgloss.NewStyle().Foreground(colorMuted)
	crumbActiveStyle := lipgloss.NewStyle().Foreground(colorHighlight).Bold(true)
	selectedBg := lipgloss.NewStyle().Foreground(colorHighlight).Bold(true)
	normalEntry := lipgloss.NewStyle().Foreground(colorFg)
	mutedEntry := lipgloss.NewStyle().Foreground(colorMuted)
	matchStyle := lipgloss.NewStyle().Foreground(colorYellow).Bold(true)
	dirArrow := lipgloss.NewStyle().Foreground(colorCyan)
	emptyStyle := lipgloss.NewStyle().Foreground(colorMuted).Italic(true)

	var b strings.Builder

	// Breadcrumb
	crumb := c.breadcrumb(innerWidth)
	if crumb != "" {
		// Split to highlight last segment
		parts := strings.Split(crumb, " / ")
		if len(parts) > 1 {
			prefix := strings.Join(parts[:len(parts)-1], crumbStyle.Render(" / "))
			last := crumbActiveStyle.Render(parts[len(parts)-1])
			b.WriteString(crumbStyle.Render(prefix) + crumbStyle.Render(" / ") + last)
		} else {
			b.WriteString(crumbActiveStyle.Render(crumb))
		}
		b.WriteString("\n")
	}

	// Handle special states
	if c.permError {
		b.WriteString(emptyStyle.Render("  (permission denied)"))
		return renderCompleterBox(b.String(), dropWidth)
	}
	if c.emptyDir {
		b.WriteString(emptyStyle.Render("  (no subdirectories)"))
		return renderCompleterBox(b.String(), dropWidth)
	}
	if c.noMatches {
		b.WriteString(emptyStyle.Render("  (no matches)"))
		return renderCompleterBox(b.String(), dropWidth)
	}
	if len(c.suggestions) == 0 {
		return ""
	}

	// Calculate visible window around selection
	total := len(c.suggestions)
	startIdx := 0
	endIdx := total
	if total > visibleRows {
		// Center the selection in the window
		startIdx = c.selected - visibleRows/2
		if startIdx < 0 {
			startIdx = 0
		}
		endIdx = startIdx + visibleRows
		if endIdx > total {
			endIdx = total
			startIdx = endIdx - visibleRows
		}
	}

	// Scroll indicator top
	if startIdx > 0 {
		b.WriteString(mutedEntry.Render("  ↑ more") + "\n")
	}

	// Suggestion rows
	for i := startIdx; i < endIdx; i++ {
		s := c.suggestions[i]
		isSelected := i == c.selected

		// Build the name with match highlighting
		var nameStr string
		if len(s.matchRanges) > 0 && !isSelected {
			nameStr = highlightMatches(s.name, s.matchRanges, matchStyle, normalEntry)
		} else if isSelected {
			nameStr = selectedBg.Render(s.name)
		} else {
			nameStr = normalEntry.Render(s.name)
		}

		// Prefix and suffix
		var prefix string
		if isSelected {
			prefix = selectedBg.Render("▸ ")
		} else {
			prefix = "  "
		}

		suffix := ""
		if s.hasChildren {
			suffix = " " + dirArrow.Render("›")
		} else {
			suffix = " " + mutedEntry.Render("·")
		}

		row := prefix + nameStr + mutedEntry.Render("/") + suffix
		b.WriteString(row)
		if i < endIdx-1 || endIdx < total {
			b.WriteString("\n")
		}
	}

	// Scroll indicator bottom
	if endIdx < total {
		b.WriteString(mutedEntry.Render("  ↓ more"))
	}

	return renderCompleterBox(b.String(), dropWidth)
}

func renderCompleterBox(content string, width int) string {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder).
		Padding(0, 1).
		Width(width - 2).
		Render(content)
}

// highlightMatches renders a string with matched character ranges highlighted.
func highlightMatches(name string, ranges [][2]int, matchSty, normalSty lipgloss.Style) string {
	if len(ranges) == 0 {
		return normalSty.Render(name)
	}

	// Build a set of matched positions
	matched := make(map[int]bool)
	for _, r := range ranges {
		for i := r[0]; i < r[1] && i < len(name); i++ {
			matched[i] = true
		}
	}

	var b strings.Builder
	inMatch := false
	start := 0

	for i := 0; i <= len(name); i++ {
		isMatch := i < len(name) && matched[i]
		if i == len(name) || isMatch != inMatch {
			// Flush segment
			segment := name[start:i]
			if segment != "" {
				if inMatch {
					b.WriteString(matchSty.Render(segment))
				} else {
					b.WriteString(normalSty.Render(segment))
				}
			}
			start = i
			inMatch = isMatch
		}
	}

	return b.String()
}

