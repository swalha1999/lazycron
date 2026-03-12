package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/swalha1999/lazycron/cron"
)

type cronFieldMode int

const (
	modeAny cronFieldMode = iota
	modeEveryN
	modeValue
)

type cronFieldModel struct {
	name     string   // "MIN", "HOUR", etc.
	min, max int
	labels   []string // optional display labels (months, weekdays)
	mode  cronFieldMode
	value int
	stepN int
}

type pickerModel struct {
	fields      [5]cronFieldModel
	activeField int
	focused     bool
}

var monthLabels = []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
var dowLabels = []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}

func newPicker() pickerModel {
	p := pickerModel{}
	p.fields[0] = cronFieldModel{name: "MIN", min: 0, max: 59, mode: modeAny, value: 0, stepN: 2}
	p.fields[1] = cronFieldModel{name: "HOUR", min: 0, max: 23, mode: modeAny, value: 0, stepN: 2}
	p.fields[2] = cronFieldModel{name: "DOM", min: 1, max: 31, mode: modeAny, value: 1, stepN: 2}
	p.fields[3] = cronFieldModel{name: "MON", min: 1, max: 12, labels: monthLabels, mode: modeAny, value: 1, stepN: 2}
	p.fields[4] = cronFieldModel{name: "DOW", min: 0, max: 6, labels: dowLabels, mode: modeAny, value: 0, stepN: 2}
	return p
}

// ParseExpression parses a cron expression into picker state.
// Returns false if the expression is too complex for the picker.
func (p *pickerModel) ParseExpression(expr string) bool {
	parts := strings.Fields(expr)
	if len(parts) != 5 {
		return false
	}

	for i, part := range parts {
		if !p.parseField(i, part) {
			return false
		}
	}
	return true
}

func (p *pickerModel) parseField(idx int, field string) bool {
	f := &p.fields[idx]

	if field == "*" {
		f.mode = modeAny
		return true
	}

	// Check for lists (commas) - too complex
	if strings.Contains(field, ",") {
		return false
	}

	// */N
	if strings.HasPrefix(field, "*/") {
		n, err := strconv.Atoi(field[2:])
		if err != nil || n < 2 {
			return false
		}
		f.mode = modeEveryN
		f.stepN = n
		return true
	}

	// Ranges, lists, complex steps — not supported in picker
	if strings.ContainsAny(field, "-/") {
		return false
	}

	// Single value
	v, err := strconv.Atoi(field)
	if err != nil {
		return false
	}
	f.mode = modeValue
	f.value = v
	return true
}

// Expression produces a cron expression from current picker state.
func (p *pickerModel) Expression() string {
	parts := make([]string, 5)
	for i := range p.fields {
		parts[i] = p.fieldExpression(i)
	}
	return strings.Join(parts, " ")
}

func (p *pickerModel) fieldExpression(idx int) string {
	f := &p.fields[idx]
	switch f.mode {
	case modeAny:
		return "*"
	case modeEveryN:
		return fmt.Sprintf("*/%d", f.stepN)
	case modeValue:
		return strconv.Itoa(f.value)
	}
	return "*"
}

func (p *pickerModel) scrollUp() {
	f := &p.fields[p.activeField]
	switch f.mode {
	case modeAny:
		f.mode = modeValue
		f.value = f.max
	case modeValue:
		f.value--
		if f.value < f.min {
			f.mode = modeEveryN
			f.stepN = f.max
		}
	case modeEveryN:
		f.stepN--
		if f.stepN < 2 {
			f.mode = modeAny
		}
	}
}

func (p *pickerModel) scrollDown() {
	f := &p.fields[p.activeField]
	switch f.mode {
	case modeAny:
		f.mode = modeEveryN
		f.stepN = 2
	case modeEveryN:
		f.stepN++
		if f.stepN > f.max {
			f.mode = modeValue
			f.value = f.min
		}
	case modeValue:
		f.value++
		if f.value > f.max {
			f.mode = modeAny
		}
	}
}

func (p *pickerModel) cycleMode() {
	f := &p.fields[p.activeField]
	switch f.mode {
	case modeAny:
		f.mode = modeEveryN
	case modeEveryN:
		f.mode = modeValue
	case modeValue:
		f.mode = modeAny
	}
}

func (p *pickerModel) moveLeft() {
	if p.activeField > 0 {
		p.activeField--
	}
}

func (p *pickerModel) moveRight() {
	if p.activeField < 4 {
		p.activeField++
	}
}

func (f *cronFieldModel) formatValue(v int) string {
	if len(f.labels) > 0 {
		idx := v - f.min
		if idx >= 0 && idx < len(f.labels) {
			return f.labels[idx]
		}
	}
	switch f.name {
	case "MIN", "HOUR":
		return fmt.Sprintf("%02d", v)
	default:
		return strconv.Itoa(v)
	}
}

func (f *cronFieldModel) prevValue(v int) int {
	v--
	if v < f.min {
		v = f.max
	}
	return v
}

func (f *cronFieldModel) nextValue(v int) int {
	v++
	if v > f.max {
		v = f.min
	}
	return v
}

func fieldDescription(name string) string {
	switch name {
	case "MIN":
		return "Minute of the hour (0-59)"
	case "HOUR":
		return "Hour of the day (0-23)"
	case "DOM":
		return "Day of the month (1-31)"
	case "MON":
		return "Month of the year (1-12)"
	case "DOW":
		return "Day of the week (0=Sun)"
	}
	return ""
}

// renderPicker renders the 5-column schedule picker.
func renderPicker(p *pickerModel, width int) string {
	colWidth := 8

	headerActive := lipgloss.NewStyle().Foreground(colorHighlight).Bold(true).Width(colWidth).Align(lipgloss.Center)
	headerInactive := lipgloss.NewStyle().Foreground(colorMuted).Width(colWidth).Align(lipgloss.Center)

	valueActive := lipgloss.NewStyle().Foreground(colorHighlight).Bold(true).Width(colWidth).Align(lipgloss.Center)
	valueInactive := lipgloss.NewStyle().Foreground(colorFg).Width(colWidth).Align(lipgloss.Center)
	valueDimmed := lipgloss.NewStyle().Foreground(colorMuted).Width(colWidth).Align(lipgloss.Center)

	aboveBelowStyle := lipgloss.NewStyle().Foreground(colorMuted).Width(colWidth).Align(lipgloss.Center)

	var headers, aboveRow, mainRow, belowRow [5]string

	for i := range p.fields {
		f := &p.fields[i]
		isActive := p.focused && i == p.activeField

		// Header
		if isActive {
			headers[i] = headerActive.Render(f.name)
		} else if p.focused {
			headers[i] = headerInactive.Render(f.name)
		} else {
			headers[i] = headerInactive.Render(f.name)
		}

		// Column content based on mode
		switch f.mode {
		case modeAny:
			aboveRow[i] = aboveBelowStyle.Render(" ")
			if isActive {
				mainRow[i] = valueActive.Render("any")
			} else if p.focused {
				mainRow[i] = valueInactive.Render("any")
			} else {
				mainRow[i] = valueDimmed.Render("any")
			}
			belowRow[i] = aboveBelowStyle.Render(" ")

		case modeEveryN:
			aboveRow[i] = aboveBelowStyle.Render(" ")
			display := fmt.Sprintf("*/%d", f.stepN)
			if isActive {
				mainRow[i] = valueActive.Render(display)
			} else if p.focused {
				mainRow[i] = valueInactive.Render(display)
			} else {
				mainRow[i] = valueDimmed.Render(display)
			}
			belowRow[i] = aboveBelowStyle.Render(" ")

		case modeValue:
			above := f.formatValue(f.prevValue(f.value))
			current := f.formatValue(f.value)
			below := f.formatValue(f.nextValue(f.value))

			if isActive {
				aboveRow[i] = aboveBelowStyle.Render(above)
				mainRow[i] = valueActive.Render("[" + current + "]")
				belowRow[i] = aboveBelowStyle.Render(below)
			} else if p.focused {
				aboveRow[i] = aboveBelowStyle.Render(above)
				mainRow[i] = valueInactive.Render(current)
				belowRow[i] = aboveBelowStyle.Render(below)
			} else {
				aboveRow[i] = aboveBelowStyle.Render(" ")
				mainRow[i] = valueDimmed.Render(current)
				belowRow[i] = aboveBelowStyle.Render(" ")
			}
		}

	}

	// Build the rows
	headerLine := strings.Join(headers[:], "")
	aboveLine := strings.Join(aboveRow[:], "")
	mainLine := strings.Join(mainRow[:], "")
	belowLine := strings.Join(belowRow[:], "")

	// Human-readable interpretation
	expr := p.Expression()
	human := cron.CronToHuman(expr)
	var humanLine string
	if human != expr {
		humanLine = lipgloss.NewStyle().Foreground(colorMuted).Render("= " + human)
	} else {
		humanLine = lipgloss.NewStyle().Foreground(colorMuted).Render("= " + expr)
	}

	// Contextual help sidebar when picker is focused
	hintStyle := lipgloss.NewStyle().Foreground(colorMuted)
	keyStyle := helpKeyStyle // cyan+bold for keys
	gap := "   "

	if p.focused {
		f := &p.fields[p.activeField]

		descLine := hintStyle.Render(fieldDescription(f.name))

		var actionLine1, actionLine2 string
		switch f.mode {
		case modeAny:
			actionLine1 = keyStyle.Render("space") + hintStyle.Render(" switch mode")
			actionLine2 = ""
		case modeEveryN:
			actionLine1 = keyStyle.Render("↑/↓") + hintStyle.Render(" change step")
			actionLine2 = keyStyle.Render("space") + hintStyle.Render(" switch mode")
		case modeValue:
			actionLine1 = keyStyle.Render("↑/↓") + hintStyle.Render(" change value")
			actionLine2 = keyStyle.Render("space") + hintStyle.Render(" switch mode")
		}

		belowHint := ""
		if actionLine2 != "" {
			belowHint = gap + actionLine2
		}

		return fmt.Sprintf("  %s%s%s\n  %s\n  %s%s%s\n  %s%s\n  %s",
			headerLine, gap, descLine,
			aboveLine,
			mainLine, gap, actionLine1,
			belowLine, belowHint,
			humanLine,
		)
	}

	return fmt.Sprintf("  %s\n  %s\n  %s\n  %s\n  %s",
		headerLine,
		aboveLine,
		mainLine,
		belowLine,
		humanLine,
	)
}
