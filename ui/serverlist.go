package ui

import (
	"fmt"
	"strings"

	"github.com/swalha1999/lazycron/backend"
)

func renderServerList(servers []backend.ServerInfo, selected, activeIdx, width, height int, focused bool) string {
	if len(servers) == 0 {
		return mutedItemStyle.Render("No servers")
	}

	var b strings.Builder

	visibleHeight := height
	startIdx := 0
	if selected >= visibleHeight {
		startIdx = selected - visibleHeight + 1
	}

	endIdx := startIdx + visibleHeight
	if endIdx > len(servers) {
		endIdx = len(servers)
	}

	for i := startIdx; i < endIdx; i++ {
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
		if i < endIdx-1 {
			b.WriteString("\n")
		}
	}

	// Scroll indicator
	if len(servers) > visibleHeight {
		scrollInfo := fmt.Sprintf(" [%d/%d]", selected+1, len(servers))
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
