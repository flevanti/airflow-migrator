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
	"github.com/flevanti/airflow-migrator/internal/core/services"
)

// Import sub-states
type importState int

const (
	importSelectFile importState = iota
	importEnterKey
	importDecrypting
	importSelectConnections
	importEnterPrefix
	importSelectProfile
	importSelectStrategy
	importConfirm
	importProcessing
	importResult
)

// importModel handles the import screen state
type importModel struct {
	state           importState
	files           []string
	fileCursor      int
	selectedFile    string
	keyInput        textinput.Model
	prefixInput     textinput.Model
	records         []*models.ExportRecord
	selected        map[string]bool
	connCursor      int
	profiles        []models.ProfileSummary
	profileCursor   int
	selectedProfile *models.Profile
	strategies      []string
	strategyCursor  int
	result          *importResultData
	err             string
	fileKey         string
}

type importResultData struct {
	imported  int
	skipped   int
	overwrote int
	errors    []string
}

func newImportModel() importModel {
	keyInput := textinput.New()
	keyInput.Placeholder = "Enter Fernet key to decrypt file"
	keyInput.EchoMode = textinput.EchoPassword
	keyInput.EchoCharacter = '•'
	keyInput.CharLimit = 256

	prefixInput := textinput.New()
	prefixInput.Placeholder = "Optional prefix (e.g., 'prod_')"
	prefixInput.CharLimit = 64

	return importModel{
		state:       importSelectFile,
		selected:    make(map[string]bool),
		keyInput:    keyInput,
		prefixInput: prefixInput,
		strategies:  []string{"skip", "overwrite", "stop"},
	}
}

func (m *Model) resetImport() {
	m.Import = newImportModel()
	m.loadCSVFiles()
	m.loadProfiles()
	m.Import.profiles = m.Profile.profiles
}

func (m *Model) loadCSVFiles() {
	m.Import.files = nil
	cwd, err := os.Getwd()
	if err != nil {
		return
	}

	entries, err := os.ReadDir(cwd)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".csv") {
			m.Import.files = append(m.Import.files, entry.Name())
		}
	}
}

func (m *Model) updateImport(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.Import.state {
	case importSelectFile:
		return m.updateImportSelectFile(msg)
	case importEnterKey:
		return m.updateImportEnterKey(msg)
	case importDecrypting:
		// Handled by async message
		return m, nil
	case importSelectConnections:
		return m.updateImportSelectConnections(msg)
	case importEnterPrefix:
		return m.updateImportEnterPrefix(msg)
	case importSelectProfile:
		return m.updateImportSelectProfile(msg)
	case importSelectStrategy:
		return m.updateImportSelectStrategy(msg)
	case importConfirm:
		return m.updateImportConfirm(msg)
	case importResult:
		return m.updateImportResult(msg)
	}
	return m, nil
}

func (m *Model) updateImportSelectFile(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			m.State = StateMainMenu
			return m, nil
		case "up", "k":
			if m.Import.fileCursor > 0 {
				m.Import.fileCursor--
			}
		case "down", "j":
			if m.Import.fileCursor < len(m.Import.files)-1 {
				m.Import.fileCursor++
			}
		case "r":
			m.loadCSVFiles()
			if m.Import.fileCursor >= len(m.Import.files) {
				m.Import.fileCursor = 0
			}
			m.Import.err = ""
			return m, nil
		case "enter":
			if len(m.Import.files) > 0 {
				m.Import.selectedFile = m.Import.files[m.Import.fileCursor]
				m.Import.state = importEnterKey
				m.Import.keyInput.Focus()
				m.Import.err = ""
				return m, nil
			}
		}
	}
	return m, nil
}

func (m *Model) updateImportEnterKey(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.Import.state = importSelectFile
			m.Import.keyInput.SetValue("")
			return m, nil
		case "enter":
			key := m.Import.keyInput.Value()
			if key == "" {
				m.Import.err = "Fernet key is required"
				return m, nil
			}
			m.Import.fileKey = key
			m.Import.state = importDecrypting
			m.Import.err = ""
			return m, m.decryptImportFile()
		}
	}

	var cmd tea.Cmd
	m.Import.keyInput, cmd = m.Import.keyInput.Update(msg)
	return m, cmd
}

type importDecryptedMsg struct {
	records []*models.ExportRecord
	err     error
}

func (m *Model) decryptImportFile() tea.Cmd {
	return func() tea.Msg {
		cwd, err := os.Getwd()
		if err != nil {
			return importDecryptedMsg{err: fmt.Errorf("failed to get current directory: %w", err)}
		}

		filePath := filepath.Join(cwd, m.Import.selectedFile)

		// Create Fernet instance
		fernet, err := services.NewFernet(m.Import.fileKey)
		if err != nil {
			return importDecryptedMsg{err: fmt.Errorf("invalid Fernet key: %w", err)}
		}

		// Read and decrypt CSV
		records, err := services.ReadEncryptedCSV(filePath, fernet)
		if err != nil {
			return importDecryptedMsg{err: fmt.Errorf("failed to decrypt file: %w", err)}
		}

		return importDecryptedMsg{records: records}
	}
}

func (m *Model) updateImportSelectConnections(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.Import.state = importEnterKey
			return m, nil
		case "up", "k":
			if m.Import.connCursor > 0 {
				m.Import.connCursor--
			}
		case "down", "j":
			if m.Import.connCursor < len(m.Import.records)-1 {
				m.Import.connCursor++
			}
		case " ":
			if len(m.Import.records) > 0 {
				connID := m.Import.records[m.Import.connCursor].ConnID
				m.Import.selected[connID] = !m.Import.selected[connID]
			}
		case "a":
			for _, r := range m.Import.records {
				m.Import.selected[r.ConnID] = true
			}
		case "n":
			for _, r := range m.Import.records {
				m.Import.selected[r.ConnID] = false
			}
		case "enter":
			selectedCount := 0
			for _, v := range m.Import.selected {
				if v {
					selectedCount++
				}
			}
			if selectedCount == 0 {
				m.Import.err = "Please select at least one connection"
				return m, nil
			}
			m.Import.state = importEnterPrefix
			m.Import.prefixInput.Focus()
			m.Import.err = ""
			return m, nil
		}
	}
	return m, nil
}

func (m *Model) updateImportEnterPrefix(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.Import.state = importSelectConnections
			return m, nil
		case "enter":
			m.Import.state = importSelectProfile
			m.Import.err = ""
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.Import.prefixInput, cmd = m.Import.prefixInput.Update(msg)
	return m, cmd
}

func (m *Model) updateImportSelectProfile(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.Import.state = importEnterPrefix
			return m, nil
		case "up", "k":
			if m.Import.profileCursor > 0 {
				m.Import.profileCursor--
			}
		case "down", "j":
			if m.Import.profileCursor < len(m.Import.profiles)-1 {
				m.Import.profileCursor++
			}
		case "enter":
			if len(m.Import.profiles) > 0 {
				profileID := m.Import.profiles[m.Import.profileCursor].ID
				m.Import.selectedProfile = m.loadFullProfile(profileID)
				if m.Import.selectedProfile == nil {
					m.Import.err = "Failed to load profile"
					return m, nil
				}
				m.Import.state = importSelectStrategy
				m.Import.err = ""
				return m, nil
			}
		}
	}
	return m, nil
}

func (m *Model) updateImportSelectStrategy(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.Import.state = importSelectProfile
			return m, nil
		case "up", "k":
			if m.Import.strategyCursor > 0 {
				m.Import.strategyCursor--
			}
		case "down", "j":
			if m.Import.strategyCursor < len(m.Import.strategies)-1 {
				m.Import.strategyCursor++
			}
		case "enter":
			m.Import.state = importConfirm
			return m, nil
		}
	}
	return m, nil
}

func (m *Model) updateImportConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.Import.state = importSelectStrategy
			return m, nil
		case "y", "Y", "enter":
			m.Import.state = importProcessing
			return m, m.performImport()
		case "n", "N":
			m.State = StateMainMenu
			m.resetImport()
			return m, nil
		}
	}
	return m, nil
}

type importCompleteMsg struct {
	result *importResultData
	err    error
}

func (m *Model) performImport() tea.Cmd {
	return func() tea.Msg {
		// Get selected connection IDs
		var selectedIDs []string
		for id, selected := range m.Import.selected {
			if selected {
				selectedIDs = append(selectedIDs, id)
			}
		}

		// Map strategy string to constant
		var strategy models.CollisionStrategy
		switch m.Import.strategies[m.Import.strategyCursor] {
		case "skip":
			strategy = models.CollisionSkip
		case "overwrite":
			strategy = models.CollisionOverwrite
		case "stop":
			strategy = models.CollisionStop
		}

		// Get current directory for file path
		cwd, err := os.Getwd()
		if err != nil {
			return importCompleteMsg{err: fmt.Errorf("failed to get current directory: %w", err)}
		}

		// Build import request
		req := models.ImportRequest{
			TargetProfile:     m.Import.selectedProfile,
			InputPath:         filepath.Join(cwd, m.Import.selectedFile),
			FileDecryptionKey: m.Import.fileKey,
			ConnectionIDs:     selectedIDs,
			ConnectionPrefix:  m.Import.prefixInput.Value(),
			CollisionStrategy: strategy,
		}

		// Perform import
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		result, err := m.Migrator.Import(ctx, req)
		if err != nil {
			return importCompleteMsg{err: err}
		}
		if !result.Success {
			return importCompleteMsg{err: fmt.Errorf("%s", result.Error)}
		}

		return importCompleteMsg{
			result: &importResultData{
				imported:  result.ImportedCount,
				skipped:   result.SkippedCount,
				overwrote: result.OverwrittenCount,
			},
		}
	}
}

func (m *Model) updateImportResult(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "enter":
			m.State = StateMainMenu
			m.resetImport()
			return m, nil
		}
	}
	return m, nil
}

func (m *Model) viewImport() string {
	switch m.Import.state {
	case importSelectFile:
		return m.viewImportSelectFile()
	case importEnterKey:
		return m.viewImportEnterKey()
	case importDecrypting:
		return m.viewImportDecrypting()
	case importSelectConnections:
		return m.viewImportSelectConnections()
	case importEnterPrefix:
		return m.viewImportEnterPrefix()
	case importSelectProfile:
		return m.viewImportSelectProfile()
	case importSelectStrategy:
		return m.viewImportSelectStrategy()
	case importConfirm:
		return m.viewImportConfirm()
	case importProcessing:
		return m.viewImportProcessing()
	case importResult:
		return m.viewImportResult()
	}
	return ""
}

func (m *Model) viewImportSelectFile() string {
	var s strings.Builder

	s.WriteString(TitleStyle.Render("📥 Import Connections"))
	s.WriteString("\n\n")
	s.WriteString("Select CSV file to import:\n\n")

	if len(m.Import.files) == 0 {
		s.WriteString(SubtleStyle.Render("No CSV files found in current directory."))
		s.WriteString("\n")
		s.WriteString(SubtleStyle.Render("Press 'r' to refresh after adding files."))
		s.WriteString("\n\n")
	} else {
		for i, f := range m.Import.files {
			cursor := "  "
			if i == m.Import.fileCursor {
				cursor = "▸ "
			}

			line := fmt.Sprintf("%s%s", cursor, f)

			if i == m.Import.fileCursor {
				s.WriteString(SelectedStyle.Render(line))
			} else {
				s.WriteString(line)
			}
			s.WriteString("\n")
		}
		s.WriteString("\n")
	}

	if m.Import.err != "" {
		s.WriteString(ErrorStyle.Render("✗ " + m.Import.err))
		s.WriteString("\n\n")
	}

	s.WriteString(SubtleStyle.Render("[Enter] select  [r]efresh  [q] back"))

	return s.String()
}

func (m *Model) viewImportEnterKey() string {
	var s strings.Builder

	s.WriteString(TitleStyle.Render("📥 Import Connections"))
	s.WriteString("\n\n")
	s.WriteString("File: ")
	s.WriteString(SelectedStyle.Render(m.Import.selectedFile))
	s.WriteString("\n\n")

	s.WriteString("Enter Fernet key to decrypt file:\n")
	s.WriteString(m.Import.keyInput.View())
	s.WriteString("\n\n")

	if m.Import.err != "" {
		s.WriteString(ErrorStyle.Render("✗ " + m.Import.err))
		s.WriteString("\n\n")
	}

	s.WriteString(SubtleStyle.Render("[Enter] decrypt  [Esc] back"))

	return s.String()
}

func (m *Model) viewImportDecrypting() string {
	var s strings.Builder

	s.WriteString(TitleStyle.Render("📥 Import Connections"))
	s.WriteString("\n\n")
	s.WriteString("Decrypting file...\n")

	return s.String()
}

func (m *Model) viewImportSelectConnections() string {
	var s strings.Builder

	s.WriteString(TitleStyle.Render("📥 Import Connections"))
	s.WriteString("\n\n")

	selectedCount := 0
	for _, v := range m.Import.selected {
		if v {
			selectedCount++
		}
	}

	s.WriteString(fmt.Sprintf("Select connections to import (%d/%d selected):\n\n",
		selectedCount, len(m.Import.records)))

	if len(m.Import.records) == 0 {
		s.WriteString(SubtleStyle.Render("No connections found in file."))
		s.WriteString("\n\n")
	} else {
		// Dynamic max visible based on terminal height
		// Reserve space for: title(2) + header(2) + footer(4) + messages(2) = ~10 lines
		maxVisible := m.Height - 10
		if maxVisible < 5 {
			maxVisible = 5
		}
		if maxVisible > len(m.Import.records) {
			maxVisible = len(m.Import.records)
		}

		startIdx := 0
		endIdx := len(m.Import.records)

		if len(m.Import.records) > maxVisible {
			startIdx = m.Import.connCursor - maxVisible/2
			if startIdx < 0 {
				startIdx = 0
			}
			endIdx = startIdx + maxVisible
			if endIdx > len(m.Import.records) {
				endIdx = len(m.Import.records)
				startIdx = endIdx - maxVisible
			}
		}

		if startIdx > 0 {
			s.WriteString(SubtleStyle.Render("    ↑ more above"))
			s.WriteString("\n\n")
		}

		for i := startIdx; i < endIdx; i++ {
			r := m.Import.records[i]
			cursor := "  "
			if i == m.Import.connCursor {
				cursor = "▸ "
			}

			checkbox := "[ ]"
			if m.Import.selected[r.ConnID] {
				checkbox = "[✓]"
			}

			line := fmt.Sprintf("%s%s %s", cursor, checkbox, r.ConnID)
			detail := fmt.Sprintf(" (%s)", r.ConnType)

			if i == m.Import.connCursor {
				s.WriteString(SelectedStyle.Render(line))
				s.WriteString(SubtleStyle.Render(detail))
			} else {
				s.WriteString(line)
				s.WriteString(SubtleStyle.Render(detail))
			}
			s.WriteString("\n")
		}

		if endIdx < len(m.Import.records) {
			s.WriteString("\n")
			s.WriteString(SubtleStyle.Render("    ↓ more below"))
		}

		s.WriteString("\n\n")
	}

	if m.Import.err != "" {
		s.WriteString(ErrorStyle.Render("✗ " + m.Import.err))
		s.WriteString("\n\n")
	}

	s.WriteString(SubtleStyle.Render("[Space] toggle  [a]ll  [n]one  [Enter] continue  [Esc] back"))

	return s.String()
}

func (m *Model) viewImportEnterPrefix() string {
	var s strings.Builder

	s.WriteString(TitleStyle.Render("📥 Import Connections"))
	s.WriteString("\n\n")

	selectedCount := 0
	for _, v := range m.Import.selected {
		if v {
			selectedCount++
		}
	}

	s.WriteString(fmt.Sprintf("Importing %d connections\n\n", selectedCount))

	s.WriteString("Enter prefix for connection IDs (optional):\n")
	s.WriteString(m.Import.prefixInput.View())
	s.WriteString("\n")
	s.WriteString(SubtleStyle.Render("(e.g., 'prod_' will rename 'my_conn' to 'prod_my_conn')"))
	s.WriteString("\n\n")

	s.WriteString(SubtleStyle.Render("[Enter] continue  [Esc] back"))

	return s.String()
}

func (m *Model) viewImportSelectProfile() string {
	var s strings.Builder

	s.WriteString(TitleStyle.Render("📥 Import Connections"))
	s.WriteString("\n\n")
	s.WriteString("Select target profile:\n\n")

	if len(m.Import.profiles) == 0 {
		s.WriteString(SubtleStyle.Render("No profiles available. Create one first."))
		s.WriteString("\n\n")
	} else {
		for i, p := range m.Import.profiles {
			cursor := "  "
			if i == m.Import.profileCursor {
				cursor = "▸ "
			}

			line := fmt.Sprintf("%s%s", cursor, p.Name)
			detail := fmt.Sprintf(" (%s/%s)", p.DBHost, p.DBName)

			if i == m.Import.profileCursor {
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

	if m.Import.err != "" {
		s.WriteString(ErrorStyle.Render("✗ " + m.Import.err))
		s.WriteString("\n\n")
	}

	s.WriteString(SubtleStyle.Render("[Enter] select  [Esc] back"))

	return s.String()
}

func (m *Model) viewImportSelectStrategy() string {
	var s strings.Builder

	s.WriteString(TitleStyle.Render("📥 Import Connections"))
	s.WriteString("\n\n")
	s.WriteString("Select collision strategy:\n\n")

	descriptions := map[string]string{
		"skip":      "Skip existing connections, import only new ones",
		"overwrite": "Overwrite existing connections with imported data",
		"stop":      "Stop import if any connection already exists",
	}

	for i, strat := range m.Import.strategies {
		cursor := "  "
		if i == m.Import.strategyCursor {
			cursor = "▸ "
		}

		line := fmt.Sprintf("%s%s", cursor, strat)

		if i == m.Import.strategyCursor {
			s.WriteString(SelectedStyle.Render(line))
			s.WriteString("\n")
			s.WriteString(SubtleStyle.Render("    " + descriptions[strat]))
		} else {
			s.WriteString(line)
		}
		s.WriteString("\n")
	}
	s.WriteString("\n")

	s.WriteString(SubtleStyle.Render("[Enter] continue  [Esc] back"))

	return s.String()
}

func (m *Model) viewImportConfirm() string {
	var s strings.Builder

	s.WriteString(TitleStyle.Render("📥 Confirm Import"))
	s.WriteString("\n\n")

	// Count selected
	selectedCount := 0
	for _, v := range m.Import.selected {
		if v {
			selectedCount++
		}
	}

	s.WriteString("Please review the import settings:\n\n")

	s.WriteString(fmt.Sprintf("  Source file:    %s\n", m.Import.selectedFile))
	s.WriteString(fmt.Sprintf("  Connections:    %d selected\n", selectedCount))

	prefix := m.Import.prefixInput.Value()
	if prefix == "" {
		prefix = "(none)"
	}
	s.WriteString(fmt.Sprintf("  Prefix:         %s\n", prefix))

	s.WriteString(fmt.Sprintf("  Target profile: %s\n", m.Import.selectedProfile.Name))
	s.WriteString(fmt.Sprintf("  Target DB:      %s/%s\n", m.Import.selectedProfile.DBHost, m.Import.selectedProfile.DBName))
	s.WriteString(fmt.Sprintf("  Strategy:       %s\n", m.Import.strategies[m.Import.strategyCursor]))

	s.WriteString("\n")
	s.WriteString("Proceed with import?\n\n")

	s.WriteString(SubtleStyle.Render("[y]es / [Enter]  [n]o  [Esc] back"))

	return s.String()
}

func (m *Model) viewImportProcessing() string {
	var s strings.Builder

	s.WriteString(TitleStyle.Render("📥 Import Connections"))
	s.WriteString("\n\n")
	s.WriteString("Importing connections...\n")

	return s.String()
}

func (m *Model) viewImportResult() string {
	var s strings.Builder

	s.WriteString(TitleStyle.Render("📥 Import Complete"))
	s.WriteString("\n\n")

	if m.Import.err != "" {
		s.WriteString(ErrorStyle.Render("✗ Import failed: " + m.Import.err))
		s.WriteString("\n\n")
	} else if m.Import.result != nil {
		s.WriteString(SuccessStyle.Render("✓ Import successful!"))
		s.WriteString("\n\n")

		s.WriteString(fmt.Sprintf("Imported:    %d\n", m.Import.result.imported))
		s.WriteString(fmt.Sprintf("Skipped:     %d\n", m.Import.result.skipped))
		s.WriteString(fmt.Sprintf("Overwritten: %d\n", m.Import.result.overwrote))
		s.WriteString("\n")
	}

	s.WriteString(SubtleStyle.Render("[Enter] done"))

	return s.String()
}
