package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/template"
	"github.com/swalha1999/lazycron/template/builtin"
)

var templatesCmd = &cobra.Command{
	Use:   "templates",
	Short: "Browse and apply job templates",
}

var templatesListCmd = &cobra.Command{
	Use:     "ls",
	Aliases: []string{"list"},
	Short: "List available templates",
	RunE:  runTemplatesList,
}

var (
	filterCategory string
)

var templatesApplyCmd = &cobra.Command{
	Use:   "apply <template-name>",
	Short: "Apply a template to create a new cron job",
	Args:  cobra.ExactArgs(1),
	RunE:  runTemplatesApply,
}

func init() {
	templatesListCmd.Flags().StringVarP(&filterCategory, "category", "c", "", "Filter by category (devops, ai, git, monitoring, system)")
	templatesCmd.AddCommand(templatesListCmd)
	templatesCmd.AddCommand(templatesApplyCmd)
	rootCmd.AddCommand(templatesCmd)
}

// templateView is the JSON shape of a template.
type templateView struct {
	Name        string `json:"name"`
	Category    string `json:"category"`
	Type        string `json:"type"`
	Schedule    string `json:"schedule"`
	Description string `json:"description"`
}

func runTemplatesList(cmd *cobra.Command, args []string) error {
	templates, err := template.LoadAll()
	if err != nil {
		return errFailure("load templates: %v", err)
	}

	if filterCategory != "" {
		cat := template.Category(filterCategory)
		var filtered []template.Template
		for _, t := range templates {
			if t.Category == cat {
				filtered = append(filtered, t)
			}
		}
		templates = filtered
	}

	views := make([]templateView, 0, len(templates))
	for _, t := range templates {
		views = append(views, templateView{
			Name: t.Name, Category: string(t.Category), Type: string(t.Type),
			Schedule: t.Schedule, Description: t.Description,
		})
	}
	return render(cmd, map[string]any{"templates": views}, func(w io.Writer) error {
		if len(templates) == 0 {
			fmt.Fprintln(w, "No templates found.")
			return nil
		}
		tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
		fmt.Fprintln(tw, "CATEGORY\tNAME\tSCHEDULE\tDESCRIPTION")
		fmt.Fprintln(tw, "--------\t----\t--------\t-----------")
		for _, t := range templates {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
				template.CategoryLabel(t.Category), t.Name, t.Schedule, truncate(t.Description, 50))
		}
		return tw.Flush()
	})
}

func runTemplatesApply(cmd *cobra.Command, args []string) error {
	name := args[0]

	templates, err := template.LoadAll()
	if err != nil {
		return fmt.Errorf("failed to load templates: %w", err)
	}

	var tmpl *template.Template
	for i := range templates {
		if strings.EqualFold(templates[i].Name, name) {
			tmpl = &templates[i]
			break
		}
	}

	if tmpl == nil {
		return errNotFound("template %q not found", name).withHint("list templates with `lazycron templates ls`")
	}

	if tmpl.Type == template.TypeSandcastle {
		if err := applySandcastleTemplate(tmpl); err != nil {
			return err
		}
		if wantJSON(cmd) {
			return writeJSON(cmd.OutOrStdout(), map[string]any{"template": tmpl.Name, "file": filepath.Join(".sandcastle", "jobs", tmpl.Filename+".ts")})
		}
		return nil
	}
	return applyYAMLTemplate(tmpl)
}

// applySandcastleTemplate copies the embedded .ts file into
// .sandcastle/jobs/<filename>.ts in the current directory. No variable
// prompts; the .ts file reads what it needs from .sandcastle/.env at runtime.
// Refuses if .sandcastle/ does not exist (the user hasn't run init yet).
func applySandcastleTemplate(tmpl *template.Template) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get cwd: %w", err)
	}
	sandcastleDir := filepath.Join(cwd, ".sandcastle")
	if _, err := os.Stat(sandcastleDir); err != nil {
		return fmt.Errorf("no .sandcastle/ directory found in %s — run `lazycron init --with-agents` first", cwd)
	}

	jobsDir := filepath.Join(sandcastleDir, "jobs")
	if err := os.MkdirAll(jobsDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", jobsDir, err)
	}

	data, err := builtin.FS.ReadFile(tmpl.SandcastleSource)
	if err != nil {
		return fmt.Errorf("read embedded template: %w", err)
	}

	dest := filepath.Join(jobsDir, tmpl.Filename+".ts")
	if _, err := os.Stat(dest); err == nil {
		return fmt.Errorf("%s already exists; remove it first if you want to re-apply", dest)
	}

	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", dest, err)
	}

	fmt.Fprintf(humanOut, "Created %s\n", filepath.Join(".sandcastle", "jobs", tmpl.Filename+".ts"))
	fmt.Fprintf(humanOut, "Schedule: %s   Name: %s\n", tmpl.Schedule, tmpl.Name)
	fmt.Fprintln(humanOut)
	fmt.Fprintln(humanOut, "Run `lazycron sync` to install the cron job.")
	return nil
}

// applyYAMLTemplate is the original behaviour: prompt for variables, render,
// and append directly to the crontab.
func applyYAMLTemplate(tmpl *template.Template) error {
	if err := cron.CheckCrontabAvailable(); err != nil {
		return err
	}

	fmt.Fprintf(humanOut, "Applying template: %s\n", tmpl.Name)
	fmt.Fprintf(humanOut, "Description: %s\n", tmpl.Description)
	fmt.Fprintln(humanOut)

	reader := bufio.NewReader(os.Stdin)
	values := make(map[string]string)

	for _, v := range tmpl.Variables {
		prompt := v.Prompt
		if v.Default != "" {
			prompt += fmt.Sprintf(" [%s]", v.Default)
		}
		fmt.Fprintf(humanOut, "%s: ", prompt)

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input != "" {
			values[v.Name] = input
		}
	}

	resolvedCmd, resolvedSchedule := tmpl.Apply(values)
	cronExpr := cron.HumanToCron(resolvedSchedule)

	fmt.Fprintf(humanOut, "Job name [%s]: ", tmpl.Name)
	jobName, _ := reader.ReadString('\n')
	jobName = strings.TrimSpace(jobName)
	if jobName == "" {
		jobName = tmpl.Name
	}

	job := cron.Job{
		Name:     jobName,
		Schedule: cronExpr,
		Command:  resolvedCmd,
		Enabled:  true,
		Wrapped:  true,
	}

	raw, err := cron.ReadCrontab()
	if err != nil {
		return fmt.Errorf("failed to read crontab: %w", err)
	}
	jobs := cron.Parse(raw)

	jobs = append(jobs, job)
	if err := cron.WriteCrontab(jobs); err != nil {
		return fmt.Errorf("failed to write crontab: %w", err)
	}

	fmt.Fprintf(humanOut, "\nCreated job '%s'\n", job.Name)
	fmt.Fprintf(humanOut, "  Schedule: %s\n", job.Schedule)
	fmt.Fprintf(humanOut, "  Command:  %s\n", job.Command)
	return nil
}
