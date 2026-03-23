package ui

import (
	"fmt"
	"strings"

	"github.com/swalha1999/lazycron/backend"
)

func renderServerList(servers []backend.ServerInfo, selected, activeIdx, width, height int, focused bool, matchSet map[int]bool) string {
	if len(servers) == 0 {
		return mutedItemStyle.Render("No servers")
	}

	// Build list of visible indices
	var visible []int
	for i := range servers {
		if matchSet == nil || matchSet[i] {
			visible = append(visible, i)
		}
	}

	if len(visible) == 0 {
		return mutedItemStyle.Render("No matches")
	}

	// Find selected position within visible list
	selPos := 0
	for j, idx := range visible {
		if idx == selected {
			selPos = j
			break
		}
	}

	var b strings.Builder

	visibleHeight := height
	startPos := 0
	if selPos >= visibleHeight {
		startPos = selPos - visibleHeight + 1
	}

	endPos := startPos + visibleHeight
	if endPos > len(visible) {
		endPos = len(visible)
	}

	for p := startPos; p < endPos; p++ {
		i := visible[p]
		srv := servers[i]
		isSelected := i == selected
		isActive := i == activeIdx

		// Cursor
		cursor := "  "
		if isSelected && focused {
			cursor = "▶ "
		} else if isActive {
			cursor = "> "
		}

		// Connection status dot
		dot := statusDot(srv.Status)

		// Name
		nameWidth := width - 6 // cursor(2) + dot(2) + padding(2)
		if nameWidth < 4 {
			nameWidth = 4
		}
		name := srv.Name
		if len(name) > nameWidth {
			name = name[:nameWidth-1] + "…"
		}
		name = fmt.Sprintf("%-*s", nameWidth, name)

		var line string
		if isSelected && focused {
			line = cursor + selectedStyle.Render(name) + " " + dot
		} else if isActive {
			line = cursor + normalStyle.Render(name) + " " + dot
		} else {
			line = cursor + mutedItemStyle.Render(name) + " " + dot
		}

		b.WriteString(line)
		if p < endPos-1 {
			b.WriteString("\n")
		}
	}

	// Scroll indicator
	if len(visible) > visibleHeight {
		scrollInfo := fmt.Sprintf(" [%d/%d]", selPos+1, len(visible))
		b.WriteString("\n" + mutedItemStyle.Render(scrollInfo))
	}

	return b.String()
}

func statusDot(status backend.ConnStatus) string {
	switch status {
	case backend.ConnLocal:
		return connectedDotStyle.Render("●")
	case backend.ConnConnected:
		return connectedDotStyle.Render("●")
	case backend.ConnConnecting:
		return connectingDotStyle.Render("●")
	case backend.ConnError:
		return connErrorDotStyle.Render("●")
	default:
		return mutedItemStyle.Render("○")
	}
}
