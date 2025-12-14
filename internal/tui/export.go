package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/flevanti/airflow-migrator/internal/core/models"
	"golang.design/x/clipboard"
)

// Export sub-states
type exportState int

const (
	exportSelectProfile exportState = iota
	exportLoadingConnections
	exportSelectConnections
	exportEnterKey
	exportProcessing
	exportResult
)

// exportModel handles the export screen state
type exportModel struct {
	state           exportState
	profiles        []models.ProfileSummary
	profileCursor   int
	selectedProfile *models.Profile
	connections     []*models.Connection
	selected        map[string]bool
	connCursor      int
	keyInput        textinput.Model
	result          *exportResultData
	err             string
	copied          bool
}

type exportResultData struct {
	filename  string
	location  string
	fernetKey string
	count     int
}

func newExportModel() exportModel {
	keyInput := textinput.New()
	keyInput.Placeholder = "Leave empty to auto-generate"
	keyInput.EchoMode = textinput.EchoPassword
	keyInput.EchoCharacter = '•'
	keyInput.CharLimit = 256

	return exportModel{
		state:    exportSelectProfile,
		selected: make(map[string]bool),
		keyInput: keyInput,
	}
}

func (m *Model) resetExport() {
	m.Export = newExportModel()
	m.loadProfiles()
	m.Export.profiles = m.Profile.profiles
}

func (m *Model) updateExport(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.Export.state {
	case exportSelectProfile:
		return m.updateExportSelectProfile(msg)
	case exportSelectConnections:
		return m.updateExportSelectConnections(msg)
	case exportEnterKey:
		return m.updateExportEnterKey(msg)
	case exportResult:
		return m.updateExportResult(msg)
	}
	return m, nil
}

func (m *Model) updateExportSelectProfile(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			m.State = StateMainMenu
			return m, nil
		case "up", "k":
			if m.Export.profileCursor > 0 {
				m.Export.profileCursor--
			}
		case "down", "j":
			if m.Export.profileCursor < len(m.Export.profiles)-1 {
				m.Export.profileCursor++
			}
		case "enter":
			if len(m.Export.profiles) > 0 {
				// Load full profile and fetch connections
				profileID := m.Export.profiles[m.Export.profileCursor].ID
				m.Export.selectedProfile = m.loadFullProfile(profileID)
				if m.Export.selectedProfile == nil {
					m.Export.err = "Failed to load profile"
					return m, nil
				}
				m.Export.state = exportLoadingConnections
				m.Export.err = ""
				// Fetch connections
				return m, m.fetchConnections()
			}
		}
	}
	return m, nil
}

// Message type for async connection fetching
type connectionsLoadedMsg struct {
	connections []*models.Connection
	err         error
}

func (m *Model) fetchConnections() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		connections, err := m.Migrator.ListConnections(ctx, m.Export.selectedProfile)
		return connectionsLoadedMsg{connections: connections, err: err}
	}
}

func (m *Model) updateExportSelectConnections(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			m.Export.state = exportSelectProfile
			return m, nil
		case "up", "k":
			if m.Export.connCursor > 0 {
				m.Export.connCursor--
			}
		case "down", "j":
			if m.Export.connCursor < len(m.Export.connections)-1 {
				m.Export.connCursor++
			}
		case " ":
			// Toggle current selection
			if len(m.Export.connections) > 0 {
				connID := m.Export.connections[m.Export.connCursor].ID
				m.Export.selected[connID] = !m.Export.selected[connID]
			}
		case "a":
			// Select all
			for _, c := range m.Export.connections {
				m.Export.selected[c.ID] = true
			}
		case "n":
			// Select none
			for _, c := range m.Export.connections {
				m.Export.selected[c.ID] = false
			}
		case "enter":
			// Check if any selected
			selectedCount := 0
			for _, v := range m.Export.selected {
				if v {
					selectedCount++
				}
			}
			if selectedCount == 0 {
				m.Export.err = "Please select at least one connection"
				return m, nil
			}
			m.Export.state = exportEnterKey
			m.Export.keyInput.Focus()
			m.Export.err = ""
			return m, nil
		}
	}
	return m, nil
}

func (m *Model) updateExportEnterKey(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.Export.state = exportSelectConnections
			return m, nil
		case "enter":
			// Perform export
			m.Export.state = exportProcessing
			return m, m.performExport()
		}
	}

	var cmd tea.Cmd
	m.Export.keyInput, cmd = m.Export.keyInput.Update(msg)
	return m, cmd
}

type exportCompleteMsg struct {
	result *exportResultData
	err    error
}

func (m *Model) performExport() tea.Cmd {
	return func() tea.Msg {
		// Get selected connection IDs
		var selectedIDs []string
		for id, selected := range m.Export.selected {
			if selected {
				selectedIDs = append(selectedIDs, id)
			}
		}

		// Get or generate Fernet key
		fernetKey := m.Export.keyInput.Value()
		if fernetKey == "" {
			var err error
			fernetKey, err = m.Migrator.GenerateFernetKey()
			if err != nil {
				return exportCompleteMsg{err: fmt.Errorf("failed to generate Fernet key: %w", err)}
			}
		}

		// Generate filename
		profileName := strings.ReplaceAll(m.Export.selectedProfile.Name, " ", "_")
		timestamp := time.Now().Format("20060102_150405")
		filename := fmt.Sprintf("airflow_%s_%s.csv", profileName, timestamp)

		// Create temp file path
		tempPath := filepath.Join(os.TempDir(), filename)

		// Build export request
		req := models.ExportRequest{
			SourceProfile:     m.Export.selectedProfile,
			OutputPath:        tempPath,
			FileEncryptionKey: fernetKey,
			ConnectionIDs:     selectedIDs,
		}

		// Perform export
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		result, err := m.Migrator.Export(ctx, req)
		if err != nil {
			return exportCompleteMsg{err: err}
		}
		if !result.Success {
			return exportCompleteMsg{err: fmt.Errorf("%s", result.Error)}
		}

		// Copy to current directory
		cwd, err := os.Getwd()
		if err != nil {
			return exportCompleteMsg{err: fmt.Errorf("failed to get current directory: %w", err)}
		}

		destPath := filepath.Join(cwd, filename)

		// Read temp file
		data, err := os.ReadFile(tempPath)
		if err != nil {
			return exportCompleteMsg{err: fmt.Errorf("failed to read temp file: %w", err)}
		}

		// Write to destination
		if err := os.WriteFile(destPath, data, 0600); err != nil {
			return exportCompleteMsg{err: fmt.Errorf("failed to write file: %w", err)}
		}

		// Clean up temp file
		os.Remove(tempPath)

		return exportCompleteMsg{
			result: &exportResultData{
				filename:  filename,
				location:  destPath,
				fernetKey: fernetKey,
				count:     result.ConnectionCount,
			},
		}
	}
}

func (m *Model) updateExportResult(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "enter":
			m.State = StateMainMenu
			m.resetExport()
			return m, nil
		case "c":
			if m.Export.result != nil && m.Export.result.fernetKey != "" {
				err := clipboard.Init()
				if err == nil {
					clipboard.Write(clipboard.FmtText, []byte(m.Export.result.fernetKey))
					m.Export.copied = true
				}
			}
			return m, nil
		}
	}
	return m, nil
}

func (m *Model) viewExport() string {
	switch m.Export.state {
	case exportSelectProfile:
		return m.viewExportSelectProfile()
	case exportLoadingConnections:
		return m.viewExportLoading()
	case exportSelectConnections:
		return m.viewExportSelectConnections()
	case exportEnterKey:
		return m.viewExportEnterKey()
	case exportProcessing:
		return m.viewExportProcessing()
	case exportResult:
		return m.viewExportResult()
	}
	return ""
}

func (m *Model) viewExportSelectProfile() string {
	var s strings.Builder

	s.WriteString(TitleStyle.Render("📤 Export Connections"))
	s.WriteString("\n\n")
	s.WriteString("Select source profile:\n\n")

	if len(m.Export.profiles) == 0 {
		s.WriteString(SubtleStyle.Render("No profiles available. Create one first."))
		s.WriteString("\n\n")
	} else {
		for i, p := range m.Export.profiles {
			cursor := "  "
			if i == m.Export.profileCursor {
				cursor = "▸ "
			}

			line := fmt.Sprintf("%s%s", cursor, p.Name)
			detail := fmt.Sprintf(" (%s/%s)", p.DBHost, p.DBName)

			if i == m.Export.profileCursor {
				s.WriteString(SelectedStyle.Render(line))
				s.WriteString(SubtleStyle.Render(detail))
			} else {
				s.WriteString(line)
				s.WriteString(SubtleStyle.Render(detail))
			}
			s.WriteString("\n")
		}
		s.WriteString("\n")
	}

	if m.Export.err != "" {
		s.WriteString(ErrorStyle.Render("✗ " + m.Export.err))
		s.WriteString("\n\n")
	}

	s.WriteString(SubtleStyle.Render("[Enter] select  [q] back"))

	return s.String()
}

func (m *Model) viewExportLoading() string {
	var s strings.Builder

	s.WriteString(TitleStyle.Render("📤 Export Connections"))
	s.WriteString("\n\n")
	s.WriteString("Loading connections from ")
	s.WriteString(SelectedStyle.Render(m.Export.selectedProfile.Name))
	s.WriteString("...\n")

	return s.String()
}

func (m *Model) viewExportSelectConnections() string {
	var s strings.Builder

	s.WriteString(TitleStyle.Render("📤 Export Connections"))
	s.WriteString("\n\n")

	// Count selected
	selectedCount := 0
	for _, v := range m.Export.selected {
		if v {
			selectedCount++
		}
	}

	s.WriteString(fmt.Sprintf("Select connections to export (%d/%d selected):\n\n",
		selectedCount, len(m.Export.connections)))

	if len(m.Export.connections) == 0 {
		s.WriteString(SubtleStyle.Render("No connections found in this database."))
		s.WriteString("\n\n")
	} else {
		// Dynamic max visible based on terminal height
		// Reserve space for: title(2) + header(2) + footer(4) + messages(2) = ~10 lines
		maxVisible := m.Height - 10
		if maxVisible < 5 {
			maxVisible = 5
		}
		if maxVisible > len(m.Export.connections) {
			maxVisible = len(m.Export.connections)
		}

		startIdx := 0
		endIdx := len(m.Export.connections)

		if len(m.Export.connections) > maxVisible {
			startIdx = m.Export.connCursor - maxVisible/2
			if startIdx < 0 {
				startIdx = 0
			}
			endIdx = startIdx + maxVisible
			if endIdx > len(m.Export.connections) {
				endIdx = len(m.Export.connections)
				startIdx = endIdx - maxVisible
			}
		}

		if startIdx > 0 {
			s.WriteString(SubtleStyle.Render("    ↑ more above"))
			s.WriteString("\n\n")
		}

		for i := startIdx; i < endIdx; i++ {
			c := m.Export.connections[i]
			cursor := "  "
			if i == m.Export.connCursor {
				cursor = "▸ "
			}

			checkbox := "[ ]"
			if m.Export.selected[c.ID] {
				checkbox = "[✓]"
			}

			line := fmt.Sprintf("%s%s %s", cursor, checkbox, c.ID)
			detail := fmt.Sprintf(" (%s)", c.ConnType)

			if i == m.Export.connCursor {
				s.WriteString(SelectedStyle.Render(line))
				s.WriteString(SubtleStyle.Render(detail))
			} else {
				s.WriteString(line)
				s.WriteString(SubtleStyle.Render(detail))
			}
			s.WriteString("\n")
		}

		if endIdx < len(m.Export.connections) {
			s.WriteString("\n")
			s.WriteString(SubtleStyle.Render("    ↓ more below"))
		}

		s.WriteString("\n\n")
	}

	if m.Export.err != "" {
		s.WriteString(ErrorStyle.Render("✗ " + m.Export.err))
		s.WriteString("\n\n")
	}

	s.WriteString(SubtleStyle.Render("[Space] toggle  [a]ll  [n]one  [Enter] continue  [Esc] back"))

	return s.String()
}

func (m *Model) viewExportEnterKey() string {
	var s strings.Builder

	s.WriteString(TitleStyle.Render("📤 Export Connections"))
	s.WriteString("\n\n")

	// Count selected
	selectedCount := 0
	for _, v := range m.Export.selected {
		if v {
			selectedCount++
		}
	}

	s.WriteString(fmt.Sprintf("Exporting %d connections from ", selectedCount))
	s.WriteString(SelectedStyle.Render(m.Export.selectedProfile.Name))
	s.WriteString("\n\n")

	s.WriteString("Enter Fernet key for file encryption:\n")
	s.WriteString(m.Export.keyInput.View())
	s.WriteString("\n")
	s.WriteString(SubtleStyle.Render("(Leave empty to auto-generate a new key)"))
	s.WriteString("\n\n")

	s.WriteString(SubtleStyle.Render("[Enter] export  [Esc] back"))

	return s.String()
}

func (m *Model) viewExportProcessing() string {
	var s strings.Builder

	s.WriteString(TitleStyle.Render("📤 Export Connections"))
	s.WriteString("\n\n")
	s.WriteString("Exporting connections...\n")

	return s.String()
}

func (m *Model) viewExportResult() string {
	var s strings.Builder

	s.WriteString(TitleStyle.Render("📤 Export Complete"))
	s.WriteString("\n\n")

	if m.Export.err != "" {
		s.WriteString(ErrorStyle.Render("✗ Export failed: " + m.Export.err))
		s.WriteString("\n\n")
	} else if m.Export.result != nil {
		s.WriteString(SuccessStyle.Render("✓ Export successful!"))
		s.WriteString("\n\n")

		s.WriteString(fmt.Sprintf("Connections exported: %d\n", m.Export.result.count))
		s.WriteString(fmt.Sprintf("Filename: %s\n", m.Export.result.filename))
		s.WriteString(fmt.Sprintf("Location: %s\n", m.Export.result.location))
		s.WriteString("\n")
		s.WriteString("Fernet Key (save this to decrypt the file):\n")
		s.WriteString(SelectedStyle.Render(m.Export.result.fernetKey))
		if m.Export.copied {
			s.WriteString("  ")
			s.WriteString(SuccessStyle.Render("✓ Copied!"))
		}
		s.WriteString("\n\n")
	}

	s.WriteString(SubtleStyle.Render("[c]opy key  [Enter] done"))

	return s.String()
}
