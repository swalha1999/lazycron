package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var logo = []string{
	`    __                                      `,
	`   / /___ _____  __  ________________  ____ `,
	`  / / __ '/_  / / / / / ___/ ___/ __ \/ __ \`,
	` / / /_/ / / /_/ /_/ / /__/ /  / /_/ / / / /`,
	`/_/\__,_/ /___/\__, /\___/_/   \____/_/ /_/ `,
	`              /____/                         `,
}

func renderSplash(version string, width, height int) string {
	logoStyle := lipgloss.NewStyle().
		Foreground(colorHighlight).
		Bold(true)

	versionStyle := lipgloss.NewStyle().
		Foreground(colorMuted)

	taglineStyle := lipgloss.NewStyle().
		Foreground(colorCyan)

	hintStyle := lipgloss.NewStyle().
		Foreground(colorMuted).
		Italic(true)

	var b strings.Builder
	for _, line := range logo {
		b.WriteString(logoStyle.Render(line))
		b.WriteString("\n")
	}
	b.WriteString(taglineStyle.Render("       automation that never sleeps") +
		versionStyle.Render("  " + version))
	b.WriteString("\n")
	b.WriteString(hintStyle.Render("          press any key to continue"))

	content := b.String()

	box := lipgloss.NewStyle().
		Padding(1, 3).
		Render(content)

	return lipgloss.Place(width, height,
		lipgloss.Center, lipgloss.Center,
		box,
	)
}
