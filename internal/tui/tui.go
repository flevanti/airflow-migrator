package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/flevanti/airflow-migrator/internal/core"
	"github.com/flevanti/airflow-migrator/internal/secrets"
)

// Styles
var (
	TitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("63")).
		MarginBottom(1)

	ErrorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("196"))

	SuccessStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("42"))

	SubtleStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("241"))

	SelectedStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("212"))
)

// App state
type AppState int

const (
	StateMainMenu AppState = iota
	StateProfiles
	StateExport
	StateImport
	StateAbout
)

// Model is the main TUI model
type Model struct {
	State     AppState
	ConfigDir string
	Secrets   *secrets.Store
	Migrator  *core.Migrator
	Width     int
	Height    int

	// Sub-models
	Profile profileModel
	Export  exportModel
	Import  importModel
}

// NewModel creates a new TUI model
func NewModel(configDir string, secrets *secrets.Store, migrator *core.Migrator) Model {
	return Model{
		State:     StateMainMenu,
		ConfigDir: configDir,
		Secrets:   secrets,
		Migrator:  migrator,
		Profile:   newProfileModel(),
		Export:    newExportModel(),
		Import:    newImportModel(),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.EnableMouseCellMotion
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle async messages first
	switch msg := msg.(type) {
	case connectionsLoadedMsg:
		if msg.err != nil {
			m.Export.err = "Failed to load connections: " + msg.err.Error()
			m.Export.state = exportSelectProfile
		} else {
			m.Export.connections = msg.connections
			m.Export.selected = make(map[string]bool)
			// Select all by default
			for _, c := range msg.connections {
				m.Export.selected[c.ID] = true
			}
			m.Export.connCursor = 0
			m.Export.state = exportSelectConnections
		}
		return m, nil

	case exportCompleteMsg:
		if msg.err != nil {
			m.Export.err = msg.err.Error()
		} else {
			m.Export.result = msg.result
		}
		m.Export.state = exportResult
		return m, nil

	case importDecryptedMsg:
		if msg.err != nil {
			m.Import.err = msg.err.Error()
			m.Import.state = importEnterKey
		} else {
			m.Import.records = msg.records
			m.Import.selected = make(map[string]bool)
			m.Import.connCursor = 0
			m.Import.state = importSelectConnections
		}
		return m, nil

	case importCompleteMsg:
		if msg.err != nil {
			m.Import.err = msg.err.Error()
		} else {
			m.Import.result = msg.result
		}
		m.Import.state = importResult
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
	}

	switch m.State {
	case StateMainMenu:
		return m.updateMainMenu(msg)
	case StateProfiles:
		return m.updateProfiles(msg)
	case StateExport:
		return m.updateExport(msg)
	case StateImport:
		return m.updateImport(msg)
	case StateAbout:
		return m.updateAbout(msg)
	}

	return m, nil
}

func (m Model) updateMainMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "1", "p":
			m.State = StateProfiles
			m.loadProfiles()
			return m, nil
		case "2", "e":
			m.State = StateExport
			m.resetExport()
			return m, nil
		case "3", "i":
			m.State = StateImport
			m.resetImport()
			return m, nil
		case "4", "a":
			m.State = StateAbout
			return m, nil
		}
	}
	return m, nil
}

func (m Model) View() string {
	switch m.State {
	case StateMainMenu:
		return m.viewMainMenu()
	case StateProfiles:
		return m.viewProfiles()
	case StateExport:
		return m.viewExport()
	case StateImport:
		return m.viewImport()
	case StateAbout:
		return m.viewAbout()
	default:
		return "Not implemented yet...\n\nPress q to quit"
	}
}

func (m Model) viewMainMenu() string {
	s := TitleStyle.Render("✈️  Airflow Connection Migrator") + "\n\n"

	s += "What would you like to do?\n\n"

	s += "  [1] 📋 Profiles     - Manage connection profiles\n"
	s += "  [2] 📤 Export       - Export connections to CSV\n"
	s += "  [3] 📥 Import       - Import connections from CSV\n"
	s += "  [4] ℹ️  About        - About this application\n\n"

	s += SubtleStyle.Render("Press number or letter • q to quit")

	return s
}
