package ui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const visibleRows = 8 // how many rows to show at once

type suggestion struct {
	name        string   // directory basename
	fullPath    string   // absolute path with trailing /
	matchRanges [][2]int // matched char ranges in name for highlighting
	hasChildren bool     // has subdirectories
}

type completerModel struct {
	suggestions []suggestion
	selected    int
	active      bool
	browseDir   string    // directory currently being listed (absolute, trailing /)
	filterText  string    // text typed after the last /
	noMatches   bool      // true when filter has no results
	permError   bool      // true when directory is unreadable
	emptyDir    bool      // true when directory has no subdirs
	lister      DirLister // filesystem abstraction (local or remote)
}

// activate initializes the completer for a given input value.
func (c *completerModel) activate(input string) {
	c.update(input)
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

// handleKey processes completer navigation keys.
// setInput is called with the selected path when the user drills in/out.
// Returns true if the key was handled by the completer.
func (c *completerModel) handleKey(key string, setInput func(string)) bool {
	switch key {
	case "down":
		c.selectNext()
	case "up":
		c.selectPrev()
	case "enter", "right":
		if c.selected >= 0 {
			if path := c.drillIn(); path != "" {
				setInput(path)
			}
		} else if key == "right" {
			// right with no selection: handled (consume the key)
		} else {
			return false // enter with no selection: let caller handle
		}
	case "left":
		setInput(c.drillOut())
	case "esc":
		c.reset()
	default:
		return false
	}
	return true
}

// parseInput splits input into browseDir and filterText.
func (c *completerModel) parseInput(input string) {
	expanded := c.expandTilde(input)
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

	if home, err := c.homeDir(); err == nil {
		seeds = append(seeds, suggestion{
			name:        "~",
			fullPath:    home + "/",
			hasChildren: true,
		})
	}

	// Only show cwd for local (it's the local machine's cwd, irrelevant for remote)
	if c.lister == nil {
		if cwd, err := os.Getwd(); err == nil {
			seeds = append(seeds, suggestion{
				name:        ".",
				fullPath:    cwd + "/",
				hasChildren: true,
			})
		}
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

	dirNames, err := c.listDirs(c.browseDir)
	if err != nil {
		c.suggestions = nil
		c.permError = true
		c.active = true
		c.selected = -1
		return
	}

	var dirs []dirEntry
	for _, name := range dirNames {
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(c.filterText, ".") {
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
			hasChildren: c.hasSubdirs(fullPath),
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

	display := c.collapseTilde(c.browseDir)
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
