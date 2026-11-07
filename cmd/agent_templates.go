package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/swalha1999/lazycron/template"
	"github.com/swalha1999/lazycron/template/builtin"
)

// errJobExists is returned by writeAgentTemplate when the job file is already there.
var errJobExists = errors.New("agent job already exists")

// agentJobPath is where an agent template lands, relative to the project.
func agentJobPath(tmpl *template.Template) string {
	return filepath.Join(agentsDirName, "jobs", tmpl.Filename+".md")
}

// writeAgentTemplate copies an embedded agent template into
// <cwd>/.lazycron/jobs/<filename>.md. An existing file is never overwritten:
// the job files are the user's to edit.
func writeAgentTemplate(cwd string, tmpl *template.Template) error {
	dest := filepath.Join(cwd, agentJobPath(tmpl))
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("%w: %s", errJobExists, agentJobPath(tmpl))
	}
	data, err := builtin.FS.ReadFile(tmpl.Source)
	if err != nil {
		return fmt.Errorf("read embedded template %s: %w", tmpl.Filename, err)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", dest, err)
	}
	fmt.Fprintf(humanOut, "Created %s\n", agentJobPath(tmpl))
	return nil
}

// builtinAgentTemplates returns the embedded agent job templates.
func builtinAgentTemplates() ([]template.Template, error) {
	all, err := template.LoadBuiltin()
	if err != nil {
		return nil, err
	}
	var agents []template.Template
	for _, t := range all {
		if t.Type == template.TypeAgent {
			agents = append(agents, t)
		}
	}
	return agents, nil
}
