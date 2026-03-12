package ui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const maxSuggestions = 8

type completerModel struct {
	suggestions []string
	selected    int
	active      bool
}

// update refreshes suggestions based on the current input value.
func (c *completerModel) update(input string) {
	input = strings.TrimSpace(input)
	if input == "" {
		c.reset()
		return
	}

	c.suggestions = listDirMatches(input)
	c.active = len(c.suggestions) > 0
	c.selected = -1
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
	c.selected--
	if c.selected < 0 {
		c.selected = len(c.suggestions) - 1
	}
}

func (c *completerModel) selectedValue() string {
	if c.selected >= 0 && c.selected < len(c.suggestions) {
		return c.suggestions[c.selected]
	}
	return ""
}

func (c *completerModel) reset() {
	c.suggestions = nil
	c.active = false
	c.selected = -1
}

// listDirMatches returns directories matching the partial path.
func listDirMatches(partial string) []string {
	expanded := expandTilde(partial)

	var dir, base string
	if strings.HasSuffix(expanded, "/") {
		dir = expanded
		base = ""
	} else {
		dir = filepath.Dir(expanded)
		base = filepath.Base(expanded)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var matches []string
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(base, ".") {
			continue // skip hidden unless user is typing a dot
		}

		isDir := entry.IsDir()
		if !isDir && entry.Type()&os.ModeSymlink != 0 {
			// Follow symlinks
			target, err := filepath.EvalSymlinks(filepath.Join(dir, name))
			if err == nil {
				if info, err := os.Stat(target); err == nil {
					isDir = info.IsDir()
				}
			}
		}

		if !isDir {
			continue
		}

		if base == "" || strings.HasPrefix(strings.ToLower(name), strings.ToLower(base)) {
			matches = append(matches, filepath.Join(dir, name)+"/")
		}

		if len(matches) >= maxSuggestions {
			break
		}
	}

	sort.Strings(matches)
	return matches
}

// expandTilde replaces leading ~ with the user's home directory.
func expandTilde(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[1:])
	}
	return path
}

// renderCompletions renders the suggestion dropdown.
func renderCompletions(c *completerModel, width int) string {
	if !c.active || len(c.suggestions) == 0 {
		return ""
	}

	selectedStyle := lipgloss.NewStyle().Foreground(colorHighlight).Bold(true)
	normalStyle := lipgloss.NewStyle().Foreground(colorMuted)

	var b strings.Builder
	for i, s := range c.suggestions {
		// Shorten home dir for display
		display := collapseTilde(s)
		if len(display) > width-4 {
			display = "..." + display[len(display)-width+7:]
		}

		if i == c.selected {
			b.WriteString("  " + selectedStyle.Render("> "+display))
		} else {
			b.WriteString("  " + normalStyle.Render("  "+display))
		}
		if i < len(c.suggestions)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// collapseTilde replaces the home directory prefix with ~ for display.
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
