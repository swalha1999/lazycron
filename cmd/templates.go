package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/template"
)

var templatesCmd = &cobra.Command{
	Use:   "templates",
	Short: "Browse and apply job templates",
}

var templatesListCmd = &cobra.Command{
	Use:     "ls",
	Aliases: []string{"list"},
	Short:   "List available templates",
	RunE:    runTemplatesList,
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

	if tmpl.Type == template.TypeAgent {
		if err := applyAgentTemplate(tmpl); err != nil {
			return err
		}
		if wantJSON(cmd) {
			return writeJSON(cmd.OutOrStdout(), map[string]any{"template": tmpl.Name, "file": agentJobPath(tmpl)})
		}
		return nil
	}
	return applyYAMLTemplate(tmpl)
}

// applyAgentTemplate copies an agent job into .lazycron/jobs/ in the
// current directory; `lazycron sync` then installs it.
func applyAgentTemplate(tmpl *template.Template) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get cwd: %w", err)
	}
	if hint := legacyDirHint(cwd); hint != "" {
		return fmt.Errorf("%s", hint)
	}
	if err := writeAgentTemplate(cwd, tmpl); err != nil {
		if errors.Is(err, errJobExists) {
			return errConflict("%v", err).withHint("remove it first to re-apply the template")
		}
		return err
	}
	fmt.Fprintf(humanOut, "Schedule: %s   Name: %s\n\n", tmpl.Schedule, tmpl.Name)
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
