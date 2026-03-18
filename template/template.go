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

// Template represents a reusable cron job recipe.
type Template struct {
	Name        string     `yaml:"name"`
	Category    Category   `yaml:"category"`
	Description string     `yaml:"description"`
	Schedule    string     `yaml:"schedule"`
	Command     string     `yaml:"command"`
	Variables   []Variable `yaml:"variables,omitempty"`
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
	return &t, nil
}

// Apply substitutes variable values into the template's command and returns
// a resolved command string. Values is a map of variable name to value.
// Missing values fall back to the variable's default.
func (t *Template) Apply(values map[string]string) string {
	cmd := t.Command
	for _, v := range t.Variables {
		val, ok := values[v.Name]
		if !ok || val == "" {
			val = v.Default
		}
		cmd = strings.ReplaceAll(cmd, "$"+v.Name, val)
	}
	return cmd
}

// ByCategory groups a slice of templates by their category.
func ByCategory(templates []Template) map[Category][]Template {
	grouped := make(map[Category][]Template)
	for _, t := range templates {
		grouped[t.Category] = append(grouped[t.Category], t)
	}
	return grouped
}
