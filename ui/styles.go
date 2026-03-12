package ui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors - lazygit inspired dark theme
	colorBg         = lipgloss.Color("#1e1e2e")
	colorFg         = lipgloss.Color("#cdd6f4")
	colorMuted      = lipgloss.Color("#6c7086")
	colorHighlight  = lipgloss.Color("#89b4fa")
	colorGreen      = lipgloss.Color("#a6e3a1")
	colorRed        = lipgloss.Color("#f38ba8")
	colorYellow     = lipgloss.Color("#f9e2af")
	colorCyan       = lipgloss.Color("#94e2d5")
	colorMauve      = lipgloss.Color("#cba6f7")
	colorBorder     = lipgloss.Color("#45475a")
	colorActiveBorder = lipgloss.Color("#89b4fa")
	colorOverlayBg  = lipgloss.Color("#313244")

	// Top bar
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorHighlight).
			PaddingLeft(1).
			PaddingRight(1)

	modeStyle = lipgloss.NewStyle().
			Foreground(colorMauve).
			PaddingLeft(1).
			PaddingRight(1)

	topBarStyle = lipgloss.NewStyle()

	// Bottom bar
	helpKeyStyle = lipgloss.NewStyle().
			Foreground(colorCyan).
			Bold(true)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	statusBarStyle = lipgloss.NewStyle().
			Foreground(colorFg).
			PaddingLeft(1)

	errorStyle = lipgloss.NewStyle().
			Foreground(colorRed).
			PaddingLeft(1).
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(colorGreen).
			PaddingLeft(1)

	// Panels
	panelStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(0, 1)

	activePanelStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.RoundedBorder()).
				BorderForeground(colorActiveBorder).
				Padding(0, 1)

	panelTitleStyle = lipgloss.NewStyle().
			Foreground(colorHighlight).
			Bold(true)

	// Job list items
	selectedStyle = lipgloss.NewStyle().
			Foreground(colorHighlight).
			Bold(true)

	normalStyle = lipgloss.NewStyle().
			Foreground(colorFg)

	mutedItemStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	enabledDotStyle = lipgloss.NewStyle().
			Foreground(colorGreen)

	disabledDotStyle = lipgloss.NewStyle().
				Foreground(colorMuted)

	// Detail panel
	detailLabelStyle = lipgloss.NewStyle().
				Foreground(colorCyan).
				Bold(true).
				Width(14)

	detailValueStyle = lipgloss.NewStyle().
				Foreground(colorFg)

	detailHeaderStyle = lipgloss.NewStyle().
				Foreground(colorYellow).
				Bold(true).
				MarginTop(1)

	// Form
	formStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorActiveBorder).
			Padding(1, 2)

	formTitleStyle = lipgloss.NewStyle().
			Foreground(colorHighlight).
			Bold(true).
			MarginBottom(1)

	formLabelStyle = lipgloss.NewStyle().
			Foreground(colorCyan).
			Bold(true)

	formActiveInputStyle = lipgloss.NewStyle().
				Foreground(colorFg).
				Background(lipgloss.Color("#45475a")).
				PaddingLeft(1).
				PaddingRight(1)

	formInactiveInputStyle = lipgloss.NewStyle().
				Foreground(colorMuted).
				PaddingLeft(1).
				PaddingRight(1)

	formPreviewStyle = lipgloss.NewStyle().
				Foreground(colorMauve).
				MarginTop(1).
				Italic(true)

	// Confirm dialog
	confirmStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(colorRed).
			Padding(1, 3).
			Align(lipgloss.Center)

	confirmTitleStyle = lipgloss.NewStyle().
				Foreground(colorRed).
				Bold(true)
)
