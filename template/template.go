package template

import (
	"fmt"
	"strings"

	"github.com/swalha1999/lazycron/agent"
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
//   - TypeYAML: plain job templates that write directly to crontab.
//   - TypeAgent: Claude agent jobs (Markdown) copied into .lazycron/jobs/
//     and installed by `lazycron sync`.
type TemplateType string

const (
	TypeYAML  TemplateType = "yaml"
	TypeAgent TemplateType = "agent"
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

	// Source is the embedded path of an agent template's .md file. Used by
	// `templates apply` and `init --with-agents` to copy it.
	Source string `yaml:"-"`

	// Filename is the basename (without extension) of the agent job file.
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

// ParseAgent builds a Template from an agent job's front matter.
func ParseAgent(data []byte, filename string) (*Template, error) {
	job, err := agent.ParseJob(data)
	if err != nil {
		return nil, fmt.Errorf("parse agent template %s: %w", filename, err)
	}
	return &Template{
		Name:        job.Name,
		Category:    CategoryAI,
		Description: job.Description,
		Schedule:    job.Cron,
		Command:     "lazycron agent .lazycron/jobs/" + filename + ".md",
		Tag:         job.Tag,
		TagColor:    job.TagColor,
		Type:        TypeAgent,
		Filename:    filename,
	}, nil
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
