package cmd

import (
	"github.com/spf13/cobra"
	"github.com/swalha1999/lazycron/backend"
	"gopkg.in/yaml.v3"
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Print the current jobs as a jobs file for `apply` (YAML; JSON with --json)",
	Args:  cobra.NoArgs,
	RunE:  runExport,
}

var exportProject string

func init() {
	exportCmd.Flags().StringVarP(&exportProject, "project", "p", "", "only jobs in this project (sets the file's project)")
	rootCmd.AddCommand(exportCmd)
}

func runExport(cmd *cobra.Command, args []string) error {
	return withBackend(func(b backend.Backend) error {
		jobs, err := b.ReadJobs()
		if err != nil {
			return errUnavailable("read jobs: %v", err)
		}
		file := jobsFile{Project: exportProject, Jobs: []jobSpec{}}
		if exportProject != "" {
			jobs = filterByProject(jobs, exportProject)
		}
		for _, j := range jobs {
			spec := specFromJob(j)
			if exportProject != "" {
				spec.Project = ""
			}
			file.Jobs = append(file.Jobs, spec)
		}
		if jsonForced(cmd) {
			return writeJSON(cmd.OutOrStdout(), file)
		}
		enc := yaml.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent(2)
		defer enc.Close()
		return enc.Encode(file)
	})
}
