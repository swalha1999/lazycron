package template

import (
	"testing"
)

func TestParse(t *testing.T) {
	yaml := `
name: Test Job
category: devops
description: A test template
schedule: "0 3 * * *"
command: "backup $DB_NAME to $DIR"
variables:
  - name: DB_NAME
    prompt: "Database name"
    default: mydb
  - name: DIR
    prompt: "Backup dir"
    default: /backups
`
	tmpl, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if tmpl.Name != "Test Job" {
		t.Errorf("expected name 'Test Job', got %q", tmpl.Name)
	}
	if tmpl.Category != CategoryDevOps {
		t.Errorf("expected category devops, got %q", tmpl.Category)
	}
	if len(tmpl.Variables) != 2 {
		t.Fatalf("expected 2 variables, got %d", len(tmpl.Variables))
	}
}

func TestParseInvalid(t *testing.T) {
	cases := []struct {
		name string
		yaml string
	}{
		{"missing name", `schedule: "0 * * * *"` + "\n" + `command: "echo hi"`},
		{"missing command", `name: Test` + "\n" + `schedule: "0 * * * *"`},
		{"missing schedule", `name: Test` + "\n" + `command: "echo hi"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(tc.yaml))
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

func TestApply(t *testing.T) {
	tmpl := &Template{
		Command: "pg_dump $DB_NAME > $DIR/backup.sql",
		Variables: []Variable{
			{Name: "DB_NAME", Default: "mydb"},
			{Name: "DIR", Default: "/backups"},
		},
	}

	// With explicit values
	result := tmpl.Apply(map[string]string{
		"DB_NAME": "production",
		"DIR":     "/mnt/backups",
	})
	if result != "pg_dump production > /mnt/backups/backup.sql" {
		t.Errorf("unexpected result: %q", result)
	}

	// Falls back to defaults
	result = tmpl.Apply(map[string]string{})
	if result != "pg_dump mydb > /backups/backup.sql" {
		t.Errorf("unexpected result with defaults: %q", result)
	}
}

func TestByCategory(t *testing.T) {
	templates := []Template{
		{Name: "A", Category: CategoryDevOps},
		{Name: "B", Category: CategoryAI},
		{Name: "C", Category: CategoryDevOps},
	}
	grouped := ByCategory(templates)
	if len(grouped[CategoryDevOps]) != 2 {
		t.Errorf("expected 2 devops templates, got %d", len(grouped[CategoryDevOps]))
	}
	if len(grouped[CategoryAI]) != 1 {
		t.Errorf("expected 1 ai template, got %d", len(grouped[CategoryAI]))
	}
}

func TestLoadBuiltin(t *testing.T) {
	templates, err := LoadBuiltin()
	if err != nil {
		t.Fatalf("LoadBuiltin failed: %v", err)
	}
	if len(templates) < 5 {
		t.Errorf("expected at least 5 built-in templates, got %d", len(templates))
	}

	// Verify all templates have required fields
	for _, tmpl := range templates {
		if tmpl.Name == "" {
			t.Error("template has empty name")
		}
		if tmpl.Category == "" {
			t.Error("template has empty category")
		}
		if tmpl.Schedule == "" {
			t.Errorf("template %q has empty schedule", tmpl.Name)
		}
		if tmpl.Command == "" {
			t.Errorf("template %q has empty command", tmpl.Name)
		}
	}
}
