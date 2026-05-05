package template

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Category represents a template category.
type Category string

const (
	CategoryDevOps     Category = "devops"
	CategoryAI         Category = "ai"
	CategoryGit        Category = "git"
	CategoryMonitoring Category = "monitoring"
	CategorySystem     Category = "system"
	CategoryLazycron   Category = "lazycron"
)

// AllCategories returns all available template categories in display order.
func AllCategories() []Category {
	return []Category{
		CategoryDevOps,
		CategoryAI,
		CategoryGit,
		CategoryMonitoring,
		CategorySystem,
		CategoryLazycron,
	}
}

// CategoryLabel returns a human-readable label for a category.
func CategoryLabel(c Category) string {
	switch c {
	case CategoryDevOps:
		return "DevOps"
	case CategoryAI:
		return "AI / LLM"
	case CategoryGit:
		return "Git / CI"
	case CategoryMonitoring:
		return "Monitoring"
	case CategorySystem:
		return "System"
	case CategoryLazycron:
		return "Lazycron"
	default:
		return string(c)
	}
}

// Variable represents a customizable parameter in a template.
type Variable struct {
	Name    string `yaml:"name"`
	Prompt  string `yaml:"prompt"`
	Default string `yaml:"default"`
}

// TemplateType distinguishes how a template is applied.
//   - TypeYAML: legacy/non-agent templates that write directly to crontab.
//   - TypeSandcastle: agent templates (TS files) scaffolded into
//     .sandcastle/jobs/ and installed by `lazycron sync`.
type TemplateType string

const (
	TypeYAML       TemplateType = "yaml"
	TypeSandcastle TemplateType = "sandcastle"
)

// Template represents a reusable cron job recipe.
type Template struct {
	Name        string     `yaml:"name"`
	Category    Category   `yaml:"category"`
	Description string     `yaml:"description"`
	Schedule    string     `yaml:"schedule"`
	Command     string     `yaml:"command"`
	Variables   []Variable `yaml:"variables,omitempty"`
	Tag         string     `yaml:"tag,omitempty"`
	TagColor    string     `yaml:"tag_color,omitempty"` // hex color, e.g. "#f38ba8"

	// Type indicates how `templates apply` should install this template.
	// Set by the loader from the source file extension; not user-authored.
	Type TemplateType `yaml:"-"`

	// SandcastleSource is the embedded path of the .ts file (for Type ==
	// TypeSandcastle). Used by `templates apply` to read and copy the file.
	SandcastleSource string `yaml:"-"`

	// Filename is the basename (without extension) used for the scaffolded
	// .ts file. Defaults to a slug of Name for sandcastle templates.
	Filename string `yaml:"-"`
}

// Parse parses a YAML template definition.
func Parse(data []byte) (*Template, error) {
	var t Template
	if err := yaml.Unmarshal(data, &t); err != nil {
		return nil, fmt.Errorf("invalid template YAML: %w", err)
	}
	if t.Name == "" {
		return nil, fmt.Errorf("template missing required field: name")
	}
	if t.Command == "" {
		return nil, fmt.Errorf("template missing required field: command")
	}
	if t.Schedule == "" {
		return nil, fmt.Errorf("template missing required field: schedule")
	}
	t.Type = TypeYAML
	return &t, nil
}

// ParseSandcastleTS builds a Template from a .ts file's metadata exports.
// Description is populated from a leading `// description: ...` comment if
// present. The Type is always TypeSandcastle.
func ParseSandcastleTS(data []byte, filename string) (*Template, error) {
	meta, err := ParseSandcastleMeta(data)
	if err != nil {
		return nil, fmt.Errorf("parse sandcastle template %s: %w", filename, err)
	}
	t := &Template{
		Name:        meta.Name,
		Category:    CategoryAI,
		Description: extractTSDescription(data),
		Schedule:    meta.Cron,
		Command:     "npx tsx .sandcastle/jobs/" + filename + ".ts",
		Tag:         meta.Tag,
		TagColor:    meta.TagColor,
		Type:        TypeSandcastle,
		Filename:    filename,
	}
	return t, nil
}

// extractTSDescription pulls a one-line description from a `// description: ...`
// comment, if present in the first ~20 lines. Returns empty string otherwise.
func extractTSDescription(data []byte) string {
	lines := strings.SplitN(string(data), "\n", 21)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "// description:") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "// description:"))
		}
	}
	return ""
}

// Apply substitutes variable values into the template's command and schedule,
// returning the resolved command and schedule strings.
// Missing values fall back to the variable's default.
func (t *Template) Apply(values map[string]string) (command, schedule string) {
	command = t.Command
	schedule = t.Schedule
	for _, v := range t.Variables {
		val, ok := values[v.Name]
		if !ok || val == "" {
			val = v.Default
		}
		command = strings.ReplaceAll(command, "$"+v.Name, val)
		schedule = strings.ReplaceAll(schedule, "$"+v.Name, val)
	}
	return command, schedule
}

// ByCategory groups a slice of templates by their category.
func ByCategory(templates []Template) map[Category][]Template {
	grouped := make(map[Category][]Template)
	for _, t := range templates {
		grouped[t.Category] = append(grouped[t.Category], t)
	}
	return grouped
}
