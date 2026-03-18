package cmd

import (
	"bufio"
	"fmt"
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
	Use:   "list",
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

func runTemplatesList(cmd *cobra.Command, args []string) error {
	templates, err := template.LoadAll()
	if err != nil {
		return fmt.Errorf("failed to load templates: %w", err)
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

	if len(templates) == 0 {
		fmt.Println("No templates found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "CATEGORY\tNAME\tSCHEDULE\tDESCRIPTION")
	fmt.Fprintln(w, "--------\t----\t--------\t-----------")

	for _, t := range templates {
		desc := t.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			template.CategoryLabel(t.Category), t.Name, t.Schedule, desc)
	}

	return w.Flush()
}

func runTemplatesApply(cmd *cobra.Command, args []string) error {
	name := args[0]

	if err := cron.CheckCrontabAvailable(); err != nil {
		return err
	}

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
		return fmt.Errorf("template %q not found", name)
	}

	fmt.Printf("Applying template: %s\n", tmpl.Name)
	fmt.Printf("Description: %s\n", tmpl.Description)
	fmt.Println()

	// Prompt for variable values
	reader := bufio.NewReader(os.Stdin)
	values := make(map[string]string)

	for _, v := range tmpl.Variables {
		prompt := v.Prompt
		if v.Default != "" {
			prompt += fmt.Sprintf(" [%s]", v.Default)
		}
		fmt.Printf("%s: ", prompt)

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input != "" {
			values[v.Name] = input
		}
	}

	resolvedCmd, resolvedSchedule := tmpl.Apply(values)
	cronExpr := cron.HumanToCron(resolvedSchedule)

	// Use template name as job name, prompt for override
	fmt.Printf("Job name [%s]: ", tmpl.Name)
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

	fmt.Printf("\nCreated job '%s'\n", job.Name)
	fmt.Printf("  Schedule: %s\n", job.Schedule)
	fmt.Printf("  Command:  %s\n", job.Command)
	return nil
}
