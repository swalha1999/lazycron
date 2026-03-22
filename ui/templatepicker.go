package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"

	"github.com/swalha1999/lazycron/template"
)

type templatePickerPhase int

const (
	phaseChooseSource templatePickerPhase = iota
	phaseChooseCategory
	phaseChooseTemplate
	phaseVariables
)

type templatePickerModel struct {
	phase     templatePickerPhase
	templates []template.Template
	grouped   map[template.Category][]template.Template

	// Category selection
	categories       []template.Category
	categorySelected int

	// Template selection within a category
	templateList     []template.Template
	templateSelected int

	// Variable input
	variableInputs []textinput.Model
	activeVariable int
	selectedTmpl   *template.Template

	// Path autocomplete for path-like variables
	completer completerModel
}

func newTemplatePicker(lister DirLister) templatePickerModel {
	all, _ := template.LoadAll()
	grouped := template.ByCategory(all)

	// Only include categories that have templates
	var categories []template.Category
	for _, cat := range template.AllCategories() {
		if len(grouped[cat]) > 0 {
			categories = append(categories, cat)
		}
	}

	return templatePickerModel{
		phase:      phaseChooseCategory,
		templates:  all,
		grouped:    grouped,
		categories: categories,
		completer:  completerModel{lister: lister},
	}
}

// selectCategory moves to template selection within the chosen category.
func (tp *templatePickerModel) selectCategory() {
	if tp.categorySelected >= len(tp.categories) {
		return
	}
	cat := tp.categories[tp.categorySelected]
	tp.templateList = tp.grouped[cat]
	tp.templateSelected = 0
	tp.phase = phaseChooseTemplate
}

// selectTemplate moves to variable input for the chosen template.
func (tp *templatePickerModel) selectTemplate() {
	if tp.templateSelected >= len(tp.templateList) {
		return
	}
	tmpl := tp.templateList[tp.templateSelected]
	tp.selectedTmpl = &tmpl

	if len(tmpl.Variables) == 0 {
		tp.phase = phaseVariables
		return
	}

	tp.variableInputs = make([]textinput.Model, len(tmpl.Variables))
	for i, v := range tmpl.Variables {
		ti := textinput.New()
		ti.Prompt = ""
		ti.Placeholder = v.Default
		ti.CharLimit = 256
		ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(colorMuted)
		ti.TextStyle = lipgloss.NewStyle().Foreground(colorFg)
		ti.Cursor.Style = lipgloss.NewStyle().Foreground(colorHighlight)
		tp.variableInputs[i] = ti
	}
	tp.activeVariable = 0
	tp.completer.reset()
	tp.phase = phaseVariables
	tp.activateCompleterForCurrentVar()
}

// isPathVariable returns true if the variable name suggests a filesystem path.
func isPathVariable(name string) bool {
	upper := strings.ToUpper(name)
	return strings.Contains(upper, "PATH") ||
		strings.Contains(upper, "DIR") ||
		strings.Contains(upper, "DIRECTORY") ||
		strings.Contains(upper, "FOLDER")
}

// activeVarIsPath reports whether the currently focused variable is path-like.
func (tp *templatePickerModel) activeVarIsPath() bool {
	if tp.selectedTmpl == nil || tp.activeVariable >= len(tp.selectedTmpl.Variables) {
		return false
	}
	return isPathVariable(tp.selectedTmpl.Variables[tp.activeVariable].Name)
}

// activateCompleterForCurrentVar activates the completer if the active variable is a path.
func (tp *templatePickerModel) activateCompleterForCurrentVar() {
	tp.completer.reset()
	if tp.activeVarIsPath() {
		tp.completer.activate(tp.variableInputs[tp.activeVariable].Value())
	}
}

// back goes to the previous phase. Returns false if already at the first phase.
func (tp *templatePickerModel) back() bool {
	switch tp.phase {
	case phaseChooseCategory:
		return false
	case phaseChooseTemplate:
		tp.phase = phaseChooseCategory
		return true
	case phaseVariables:
		tp.phase = phaseChooseTemplate
		return true
	}
	return false
}

// buildValues returns the variable values entered by the user.
func (tp *templatePickerModel) buildValues() map[string]string {
	values := make(map[string]string)
	if tp.selectedTmpl == nil {
		return values
	}
	for i, v := range tp.selectedTmpl.Variables {
		if i < len(tp.variableInputs) {
			val := strings.TrimSpace(tp.variableInputs[i].Value())
			if val != "" {
				values[v.Name] = val
			}
		}
	}
	return values
}

// renderTemplateName renders a template name with its optional colored tag.
// It uses an inline copy of the base style (no margins) so the tag stays
// on the same line as the name.
func renderTemplateName(tmpl template.Template, base lipgloss.Style) string {
	inline := base.Copy().UnsetMarginBottom().UnsetMarginTop()
	name := inline.Render(tmpl.Name)
	if tmpl.Tag == "" {
		return name
	}
	color := tmpl.TagColor
	if color == "" {
		color = string(colorRed)
	}
	tag := lipgloss.NewStyle().
		Foreground(lipgloss.Color(color)).
		Bold(true).
		Render(tmpl.Tag)
	return name + " " + tag
}

// renderNewJobChoice renders the "blank vs template" choice dialog.
func renderNewJobChoice(width int) string {
	formWidth := width - 10
	if formWidth > 50 {
		formWidth = 50
	}
	if formWidth < 40 {
		formWidth = 40
	}

	var b strings.Builder
	b.WriteString(formTitleStyle.Render("  New Job"))
	b.WriteString("\n\n")
	b.WriteString(selectedStyle.Render("  [b]  Blank job"))
	b.WriteString("\n")
	b.WriteString(mutedItemStyle.Render("       Start from scratch"))
	b.WriteString("\n\n")
	b.WriteString(selectedStyle.Render("  [t]  From template"))
	b.WriteString("\n")
	b.WriteString(mutedItemStyle.Render("       Pick a pre-built recipe"))
	b.WriteString("\n\n")
	b.WriteString("  " +
		helpBinding("b", "blank") + helpSep() +
		helpBinding("t", "template") + helpSep() +
		helpBinding("esc", "cancel"))

	return formStyle.Width(formWidth).Render(b.String())
}

// renderTemplatePicker renders the current phase of the template picker.
func renderTemplatePicker(tp *templatePickerModel, width int) string {
	formWidth := width - 10
	if formWidth > 70 {
		formWidth = 70
	}
	if formWidth < 40 {
		formWidth = 40
	}

	var b strings.Builder

	switch tp.phase {
	case phaseChooseCategory:
		b.WriteString(formTitleStyle.Render("  New from Template"))
		b.WriteString("\n")
		b.WriteString(mutedItemStyle.Render("  Select a category"))
		b.WriteString("\n\n")

		for i, cat := range tp.categories {
			count := len(tp.grouped[cat])
			label := fmt.Sprintf("  %s  (%d)", template.CategoryLabel(cat), count)
			if i == tp.categorySelected {
				b.WriteString(selectedStyle.Render("> "+label[2:]))
			} else {
				b.WriteString(normalStyle.Render(label))
			}
			b.WriteString("\n")
		}

		b.WriteString("\n")
		b.WriteString("  " +
			helpBinding("↑/↓", "select") + helpSep() +
			helpBinding("enter", "open") + helpSep() +
			helpBinding("esc", "cancel"))

	case phaseChooseTemplate:
		cat := tp.categories[tp.categorySelected]
		b.WriteString(formTitleStyle.Render("  " + template.CategoryLabel(cat) + " Templates"))
		b.WriteString("\n\n")

		for i, tmpl := range tp.templateList {
			desc := tmpl.Description
			if len(desc) > formWidth-10 {
				desc = desc[:formWidth-13] + "..."
			}
			if i == tp.templateSelected {
				b.WriteString("> " + renderTemplateName(tmpl, selectedStyle))
				b.WriteString("\n")
				b.WriteString("    " + mutedItemStyle.Render(desc))
			} else {
				b.WriteString("  " + renderTemplateName(tmpl, normalStyle))
				b.WriteString("\n")
				b.WriteString("    " + mutedItemStyle.Render(desc))
			}
			b.WriteString("\n")
		}

		b.WriteString("\n")
		b.WriteString("  " +
			helpBinding("↑/↓", "select") + helpSep() +
			helpBinding("enter", "choose") + helpSep() +
			helpBinding("esc", "back"))

	case phaseVariables:
		if tp.selectedTmpl == nil {
			break
		}
		b.WriteString("  " + renderTemplateName(*tp.selectedTmpl, formTitleStyle))
		b.WriteString("\n")
		b.WriteString(mutedItemStyle.Render("  " + tp.selectedTmpl.Description))
		b.WriteString("\n\n")

		if len(tp.selectedTmpl.Variables) == 0 {
			b.WriteString(normalStyle.Render("  No variables to configure."))
			b.WriteString("\n\n")
			b.WriteString("  " +
				helpBinding("enter", "create job") + helpSep() +
				helpBinding("esc", "back"))
			break
		}

		inputWidth := formWidth - 20
		for i, v := range tp.selectedTmpl.Variables {
			label := formLabelStyle.Render(fmt.Sprintf("  %-14s", v.Prompt+":"))
			tp.variableInputs[i].Width = inputWidth
			rendered := lipgloss.NewStyle().
				PaddingLeft(1).
				PaddingRight(1).
				Render(tp.variableInputs[i].View())
			b.WriteString(label + " " + rendered)
			b.WriteString("\n")

			// Show autocomplete dropdown below the active path variable
			if i == tp.activeVariable && tp.completer.active && isPathVariable(v.Name) {
				b.WriteString(renderCompletions(&tp.completer, inputWidth))
				b.WriteString("\n")
			}
		}

		b.WriteString("\n")
		completerActive := tp.activeVarIsPath() && tp.completer.active
		if completerActive {
			b.WriteString("  " +
				helpBinding("↑/↓", "select") + helpSep() +
				helpBinding("→/enter", "open") + helpSep() +
				helpBinding("←", "parent") + helpSep() +
				helpBinding("tab", "next") + helpSep() +
				helpBinding("esc", "close"))
		} else {
			b.WriteString("  " +
				helpBinding("tab", "next") + helpSep() +
				helpBinding("shift+tab", "prev") + helpSep() +
				helpBinding("enter", "create job") + helpSep() +
				helpBinding("esc", "back"))
		}
	}

	return formStyle.Width(formWidth).Render(b.String())
}
