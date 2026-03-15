package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/swalha1999/lazycron/cron"
	"github.com/swalha1999/lazycron/record"
	"github.com/swalha1999/lazycron/ui"
)

func main() {
	if err := cron.CheckCrontabAvailable(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Auto-install/update the record script
	if err := record.InstallRecord(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not install record script: %v\n", err)
	}

	p := tea.NewProgram(
		ui.NewModel(),
		tea.WithAltScreen(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
