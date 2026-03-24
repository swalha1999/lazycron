package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/swalha1999/lazycron/cron"
)

// rowKind distinguishes between project header rows and job rows in the list.
type rowKind int

const (
	rowJob    rowKind = iota
	rowHeader         // collapsible project header
)

// listRow represents a single visual row in the grouped job list.
type listRow struct {
	kind    rowKind
	jobIdx  int    // index into []cron.Job (only valid when kind == rowJob)
	project string // project name (for headers and jobs; "" means ungrouped)
}

// projectGroup holds the group name and the original indices of its jobs.
type projectGroup struct {
	name    string
	jobIdxs []int
}

// buildRows constructs the visual row list from jobs grouped by project.
// Projects are sorted alphabetically, with the "Ungrouped" section at the bottom.
// If matchSet is non-nil, only jobs with indices in matchSet are included.
func buildRows(jobs []cron.Job, collapsed map[string]bool, matchSet map[int]bool) []listRow {
	groups := groupByProject(jobs)
	var rows []listRow
	for _, g := range groups {
		if matchSet != nil {
			hasMatch := false
			for _, idx := range g.jobIdxs {
				if matchSet[idx] {
					hasMatch = true
					break
				}
			}
			if !hasMatch {
				continue
			}
		}
		rows = append(rows, listRow{kind: rowHeader, project: g.name})
		if !collapsed[g.name] {
			for _, idx := range g.jobIdxs {
				if matchSet != nil && !matchSet[idx] {
					continue
				}
				rows = append(rows, listRow{kind: rowJob, jobIdx: idx, project: g.name})
			}
		}
	}
	return rows
}

// groupByProject groups jobs by their Project field.
// Named projects are sorted alphabetically; ungrouped jobs appear last.
func groupByProject(jobs []cron.Job) []projectGroup {
	groupMap := make(map[string][]int)
	for i, j := range jobs {
		groupMap[j.Project] = append(groupMap[j.Project], i)
	}

	var named []string
	for name := range groupMap {
		if name != "" {
			named = append(named, name)
		}
	}
	sort.Strings(named)

	var groups []projectGroup
	for _, name := range named {
		groups = append(groups, projectGroup{name: name, jobIdxs: groupMap[name]})
	}
	if idxs, ok := groupMap[""]; ok {
		groups = append(groups, projectGroup{name: "", jobIdxs: idxs})
	}
	return groups
}

// rowForJobIdx finds the visual row index for a given job index, or 0 if not found.
func rowForJobIdx(rows []listRow, jobIdx int) int {
	for i, r := range rows {
		if r.kind == rowJob && r.jobIdx == jobIdx {
			return i
		}
	}
	return 0
}

// findSiblingJob finds the next job index within the same project group.
// direction is -1 for up, +1 for down. Returns -1 if no sibling exists.
func findSiblingJob(jobs []cron.Job, jobIdx int, direction int) int {
	project := jobs[jobIdx].Project
	// Collect indices of jobs in the same project, in slice order
	var siblings []int
	for i, j := range jobs {
		if j.Project == project {
			siblings = append(siblings, i)
		}
	}
	// Find position of jobIdx within siblings
	pos := -1
	for i, idx := range siblings {
		if idx == jobIdx {
			pos = i
			break
		}
	}
	if pos < 0 {
		return -1
	}
	targetPos := pos + direction
	if targetPos < 0 || targetPos >= len(siblings) {
		return -1
	}
	return siblings[targetPos]
}

func renderJobList(jobs []cron.Job, selRow int, rows []listRow, width, height int, collapsed map[string]bool, lastRunStatus map[string]*bool) string {
	if len(jobs) == 0 {
		empty := mutedItemStyle.Render("No cron jobs found")
		hint := mutedItemStyle.Render("Press 'n' to create one")
		return fmt.Sprintf("\n  %s\n\n  %s", empty, hint)
	}

	var b strings.Builder
	maxNameWidth := 0
	for _, job := range jobs {
		if len(job.Name) > maxNameWidth {
			maxNameWidth = len(job.Name)
		}
	}
	if maxNameWidth > width-20 {
		maxNameWidth = width - 20
	}
	if maxNameWidth < 8 {
		maxNameWidth = 8
	}

	// Calculate visible area for scrolling
	listHeight := height - 2
	if listHeight < 1 {
		listHeight = 1
	}

	startIdx := 0
	if selRow >= listHeight {
		startIdx = selRow - listHeight + 1
	}
	endIdx := startIdx + listHeight
	if endIdx > len(rows) {
		endIdx = len(rows)
	}

	for i := startIdx; i < endIdx; i++ {
		row := rows[i]

		if row.kind == rowHeader {
			line := renderGroupHeader(row.project, jobs, collapsed)
			if i == selRow {
				line = selectedStyle.Render("▶ " + line)
			} else {
				line = normalStyle.Render("  " + line)
			}
			b.WriteString(line)
		} else {
			job := jobs[row.jobIdx]
			line := renderJobRow(job, maxNameWidth, lastRunStatus)
			if i == selRow {
				line = selectedStyle.Render("▶ " + line)
			} else {
				line = normalStyle.Render("  " + line)
			}
			b.WriteString(line)
		}

		if i < endIdx-1 {
			b.WriteString("\n")
		}
	}

	// Scroll indicator
	if len(rows) > listHeight {
		scrollInfo := fmt.Sprintf(" %d/%d", selRow+1, len(rows))
		b.WriteString("\n")
		b.WriteString(mutedItemStyle.Render(scrollInfo))
	}

	return b.String()
}

func renderGroupHeader(project string, jobs []cron.Job, collapsed map[string]bool) string {
	displayName := project
	if project == "" {
		displayName = "Ungrouped"
	}

	count := 0
	for _, j := range jobs {
		if j.Project == project {
			count++
		}
	}

	arrow := "▼"
	if collapsed[project] {
		arrow = "▶"
	}

	header := fmt.Sprintf("%s %s (%d)", arrow, displayName, count)
	return lipgloss.NewStyle().Foreground(colorMauve).Bold(true).Render(header)
}

func renderJobRow(job cron.Job, maxNameWidth int, lastRunStatus map[string]*bool) string {
	// Status dot
	dot := enabledDotStyle.Render("●")
	if !job.Enabled {
		dot = disabledDotStyle.Render("○")
	}

	// Name (truncated) with inline tag and badge
	name := job.Name
	if len(name) > maxNameWidth {
		name = name[:maxNameWidth-1] + "…"
	}

	// Last-run health indicator
	healthIndicator := ""
	if s, ok := lastRunStatus[job.Name]; ok && s != nil {
		if *s {
			healthIndicator = " " + lipgloss.NewStyle().Foreground(colorGreen).Render("✓")
		} else {
			healthIndicator = " " + lipgloss.NewStyle().Foreground(colorRed).Bold(true).Render("✗")
		}
	}

	// Build inline badges right after the name
	nameWithBadges := name + healthIndicator
	if job.Tag != "" {
		tagColor := job.TagColor
		if tagColor == "" {
			tagColor = string(colorRed)
		}
		nameWithBadges += " " + lipgloss.NewStyle().
			Foreground(lipgloss.Color(tagColor)).
			Bold(true).
			Render(job.Tag)
	}
	if job.OneShot {
		nameWithBadges += " " + lipgloss.NewStyle().Foreground(colorYellow).Bold(true).Render("ONCE")
	}
	if !job.Wrapped {
		nameWithBadges += " " + warnStyle.Render("⚠")
	}

	// Schedule (human readable)
	schedule := cron.CronToHuman(job.Schedule)
	if len(schedule) > 24 {
		schedule = schedule[:23] + "…"
	}

	return fmt.Sprintf("   %s %s  %s", dot, nameWithBadges, mutedItemStyle.Render(schedule))
}
