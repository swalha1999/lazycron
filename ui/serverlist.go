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

	win := computeScrollWindow(len(servers), matchSet, selected, height)
	if len(win.visible) == 0 {
		return mutedItemStyle.Render("No matches")
	}

	var b strings.Builder

	for p := win.startPos; p < win.endPos; p++ {
		i := win.visible[p]
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
		if p < win.endPos-1 {
			b.WriteString("\n")
		}
	}

	if win.needsIndicator {
		scrollInfo := fmt.Sprintf(" [%d/%d]", win.selPos+1, len(win.visible))
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
