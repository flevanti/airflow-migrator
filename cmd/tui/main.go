package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/flevanti/airflow-migrator/internal/app"
	"github.com/flevanti/airflow-migrator/internal/tui"
)

func main() {
	// Initialize app (password prompt happens here, before TUI)
	application, err := app.Initialize()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Create TUI model
	model := tui.NewModel(
		application.ConfigDir,
		application.Secrets,
		application.Migrator,
	)

	// Run the TUI
	p := tea.NewProgram(
		model,
		tea.WithAltScreen(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
