package tui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/flevanti/airflow-migrator/internal/app"
)

func (m *Model) updateAbout(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "enter":
			m.State = StateMainMenu
			return m, nil
		}
	}
	return m, nil
}

func (m *Model) viewAbout() string {
	var s strings.Builder

	s.WriteString(TitleStyle.Render("ℹ️  About"))
	s.WriteString("\n\n")

	s.WriteString(SelectedStyle.Render(app.Name))
	s.WriteString("\n")
	s.WriteString(fmt.Sprintf("Version %s (Build %s)\n", app.Version, app.Build))
	s.WriteString("\n")

	s.WriteString("Configuration:\n")
	s.WriteString(fmt.Sprintf("  Config directory: %s\n", m.ConfigDir))
	s.WriteString(fmt.Sprintf("  Temp folder:      %s\n", os.TempDir()))
	s.WriteString("\n")

	s.WriteString("Links:\n")
	s.WriteString(fmt.Sprintf("  🔗 GitHub:        %s\n", app.GitHub))
	s.WriteString(fmt.Sprintf("  🐛 Issues:        %s\n", app.GitHubIssues))
	s.WriteString(fmt.Sprintf("  🔀 Pull Requests: %s\n", app.GitHubPR))
	s.WriteString("\n")

	s.WriteString(fmt.Sprintf("Developer: %s (%s)\n", app.DevName, app.DevEmail))
	s.WriteString(fmt.Sprintf("License: %s\n", app.License))
	s.WriteString(fmt.Sprintf("© %d\n", app.CurrentYear()))
	s.WriteString("\n")

	s.WriteString(SubtleStyle.Render("[Enter] back"))

	return s.String()
}
