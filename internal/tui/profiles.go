package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/flevanti/airflow-migrator/internal/core/models"
)

// Profile sub-states
type profileState int

const (
	profileList profileState = iota
	profileAdd
	profileEdit
	profileDelete
)

// Profile form fields
const (
	fieldName = iota
	fieldHost
	fieldPort
	fieldDBName
	fieldUser
	fieldPassword
	fieldFernet
	fieldCount
)

// profileModel handles the profiles screen state
type profileModel struct {
	state       profileState
	profiles    []models.ProfileSummary
	cursor      int
	inputs      []textinput.Model
	focusIndex  int
	editingID   string
	deleteID    string
	message     string
	messageType string
}

func newProfileModel() profileModel {
	inputs := make([]textinput.Model, fieldCount)

	for i := range inputs {
		t := textinput.New()
		t.CharLimit = 256
		switch i {
		case fieldName:
			t.Placeholder = "Profile Name"
			t.Focus()
		case fieldHost:
			t.Placeholder = "localhost"
		case fieldPort:
			t.Placeholder = "5432"
		case fieldDBName:
			t.Placeholder = "airflow"
		case fieldUser:
			t.Placeholder = "airflow"
		case fieldPassword:
			t.Placeholder = "password (leave empty to keep existing)"
			t.EchoMode = textinput.EchoPassword
			t.EchoCharacter = '•'
		case fieldFernet:
			t.Placeholder = "Fernet key (leave empty to generate)"
			t.EchoMode = textinput.EchoPassword
			t.EchoCharacter = '•'
		}
		inputs[i] = t
	}

	return profileModel{
		state:  profileList,
		inputs: inputs,
	}
}

func (m *Model) loadProfiles() {
	var profiles []models.ProfileSummary

	for _, key := range m.Secrets.List() {
		if strings.HasPrefix(key, "profile:") && strings.HasSuffix(key, ":meta") {
			if metaJSON, err := m.Secrets.Get(key); err == nil {
				var data struct {
					ID     string `json:"id"`
					Name   string `json:"name"`
					DBHost string `json:"db_host"`
					DBPort int    `json:"db_port"`
					DBName string `json:"db_name"`
				}
				if err := json.Unmarshal([]byte(metaJSON), &data); err == nil {
					profiles = append(profiles, models.ProfileSummary{
						ID:     data.ID,
						Name:   data.Name,
						DBHost: data.DBHost,
						DBPort: data.DBPort,
						DBName: data.DBName,
					})
				}
			}
		}
	}

	sort.Slice(profiles, func(i, j int) bool {
		return strings.ToLower(profiles[i].Name) < strings.ToLower(profiles[j].Name)
	})

	m.Profile.profiles = profiles
}

func (m *Model) updateProfiles(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.Profile.state {
	case profileList:
		return m.updateProfileList(msg)
	case profileAdd, profileEdit:
		return m.updateProfileForm(msg)
	case profileDelete:
		return m.updateProfileDelete(msg)
	}
	return m, nil
}

func (m *Model) updateProfileList(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc":
			m.State = StateMainMenu
			m.Profile.message = ""
			return m, nil
		case "up", "k":
			if m.Profile.cursor > 0 {
				m.Profile.cursor--
			}
		case "down", "j":
			if m.Profile.cursor < len(m.Profile.profiles)-1 {
				m.Profile.cursor++
			}
		case "a", "n":
			m.Profile.state = profileAdd
			m.Profile.editingID = ""
			m.resetProfileForm()
			return m, nil
		case "e", "enter":
			if len(m.Profile.profiles) > 0 {
				m.Profile.state = profileEdit
				m.Profile.editingID = m.Profile.profiles[m.Profile.cursor].ID
				m.loadProfileIntoForm(m.Profile.editingID)
				return m, nil
			}
		case "d", "backspace":
			if len(m.Profile.profiles) > 0 {
				m.Profile.state = profileDelete
				m.Profile.deleteID = m.Profile.profiles[m.Profile.cursor].ID
				return m, nil
			}
		case "t":
			if len(m.Profile.profiles) > 0 {
				m.testProfileConnection(m.Profile.profiles[m.Profile.cursor].ID)
				return m, nil
			}
		case "r":
			m.loadProfiles()
			m.Profile.message = "Refreshed"
			m.Profile.messageType = "success"
			return m, nil
		}
	}
	return m, nil
}

func (m *Model) updateProfileForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.Profile.state = profileList
			m.Profile.message = ""
			return m, nil
		case "tab", "down":
			m.Profile.focusIndex++
			if m.Profile.focusIndex >= fieldCount {
				m.Profile.focusIndex = 0
			}
			return m, m.updateProfileFocus()
		case "shift+tab", "up":
			m.Profile.focusIndex--
			if m.Profile.focusIndex < 0 {
				m.Profile.focusIndex = fieldCount - 1
			}
			return m, m.updateProfileFocus()
		case "ctrl+s":
			m.saveProfile()
			return m, nil
		case "ctrl+g":
			if key, err := m.Migrator.GenerateFernetKey(); err == nil {
				m.Profile.inputs[fieldFernet].SetValue(key)
				m.Profile.message = "Fernet key generated"
				m.Profile.messageType = "success"
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.Profile.inputs[m.Profile.focusIndex], cmd = m.Profile.inputs[m.Profile.focusIndex].Update(msg)
	return m, cmd
}

func (m *Model) updateProfileDelete(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y":
			m.deleteProfile(m.Profile.deleteID)
			m.Profile.state = profileList
			m.loadProfiles()
			if m.Profile.cursor >= len(m.Profile.profiles) && m.Profile.cursor > 0 {
				m.Profile.cursor--
			}
			return m, nil
		case "n", "N", "esc":
			m.Profile.state = profileList
			return m, nil
		}
	}
	return m, nil
}

func (m *Model) updateProfileFocus() tea.Cmd {
	cmds := make([]tea.Cmd, len(m.Profile.inputs))
	for i := range m.Profile.inputs {
		if i == m.Profile.focusIndex {
			cmds[i] = m.Profile.inputs[i].Focus()
		} else {
			m.Profile.inputs[i].Blur()
		}
	}
	return tea.Batch(cmds...)
}

func (m *Model) resetProfileForm() {
	for i := range m.Profile.inputs {
		m.Profile.inputs[i].SetValue("")
	}
	m.Profile.inputs[fieldPort].SetValue("5432")
	m.Profile.focusIndex = 0
	m.Profile.inputs[0].Focus()
	for i := 1; i < len(m.Profile.inputs); i++ {
		m.Profile.inputs[i].Blur()
	}
	m.Profile.message = ""
}

func (m *Model) loadProfileIntoForm(id string) {
	profile := m.loadFullProfile(id)
	if profile == nil {
		return
	}

	m.Profile.inputs[fieldName].SetValue(profile.Name)
	m.Profile.inputs[fieldHost].SetValue(profile.DBHost)
	m.Profile.inputs[fieldPort].SetValue(fmt.Sprintf("%d", profile.DBPort))
	m.Profile.inputs[fieldDBName].SetValue(profile.DBName)
	m.Profile.inputs[fieldUser].SetValue(profile.DBUser)
	m.Profile.inputs[fieldPassword].SetValue("")
	m.Profile.inputs[fieldFernet].SetValue("")

	m.Profile.focusIndex = 0
	m.Profile.inputs[0].Focus()
	for i := 1; i < len(m.Profile.inputs); i++ {
		m.Profile.inputs[i].Blur()
	}
	m.Profile.message = ""
}

func (m *Model) loadFullProfile(id string) *models.Profile {
	metaKey := "profile:" + id + ":meta"
	metaJSON, err := m.Secrets.Get(metaKey)
	if err != nil {
		return nil
	}

	var data struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		DBHost string `json:"db_host"`
		DBPort int    `json:"db_port"`
		DBName string `json:"db_name"`
		DBUser string `json:"db_user"`
	}

	if err := json.Unmarshal([]byte(metaJSON), &data); err != nil {
		return nil
	}

	profile := &models.Profile{
		ID:     data.ID,
		Name:   data.Name,
		DBHost: data.DBHost,
		DBPort: data.DBPort,
		DBName: data.DBName,
		DBUser: data.DBUser,
	}

	if pwd, err := m.Secrets.Get("profile:" + id + ":password"); err == nil {
		profile.DBPassword = pwd
	}
	if fernet, err := m.Secrets.Get("profile:" + id + ":fernet"); err == nil {
		profile.FernetKey = fernet
	}

	return profile
}

func (m *Model) saveProfile() {
	name := m.Profile.inputs[fieldName].Value()
	host := m.Profile.inputs[fieldHost].Value()
	portStr := m.Profile.inputs[fieldPort].Value()
	dbName := m.Profile.inputs[fieldDBName].Value()
	user := m.Profile.inputs[fieldUser].Value()
	password := m.Profile.inputs[fieldPassword].Value()
	fernet := m.Profile.inputs[fieldFernet].Value()

	// Validate required fields
	if name == "" {
		m.Profile.message = "Name is required"
		m.Profile.messageType = "error"
		return
	}
	if host == "" {
		m.Profile.message = "Host is required"
		m.Profile.messageType = "error"
		return
	}
	if dbName == "" {
		m.Profile.message = "Database name is required"
		m.Profile.messageType = "error"
		return
	}
	if user == "" {
		m.Profile.message = "User is required"
		m.Profile.messageType = "error"
		return
	}

	port := 5432
	if portStr != "" {
		fmt.Sscanf(portStr, "%d", &port)
	}

	id := m.Profile.editingID
	if id == "" {
		id = fmt.Sprintf("%d", time.Now().UnixNano())
	}

	// If editing, keep existing secrets if not provided
	if m.Profile.editingID != "" {
		existing := m.loadFullProfile(m.Profile.editingID)
		if existing != nil {
			if password == "" {
				password = existing.DBPassword
			}
			if fernet == "" {
				fernet = existing.FernetKey
			}
		}
	}

	// Generate fernet if still empty
	if fernet == "" {
		if key, err := m.Migrator.GenerateFernetKey(); err == nil {
			fernet = key
		}
	}

	// Validate password for new profiles
	if m.Profile.editingID == "" && password == "" {
		m.Profile.message = "Password is required for new profiles"
		m.Profile.messageType = "error"
		return
	}

	// Save metadata
	metaJSON := fmt.Sprintf(`{"id":"%s","name":"%s","db_host":"%s","db_port":%d,"db_name":"%s","db_user":"%s"}`,
		id, name, host, port, dbName, user)
	m.Secrets.Set("profile:"+id+":meta", metaJSON)
	m.Secrets.Set("profile:"+id+":password", password)
	m.Secrets.Set("profile:"+id+":fernet", fernet)

	m.Profile.message = "Profile saved successfully"
	m.Profile.messageType = "success"
	m.Profile.state = profileList
	m.loadProfiles()
}

func (m *Model) deleteProfile(id string) {
	m.Secrets.Delete("profile:" + id + ":meta")
	m.Secrets.Delete("profile:" + id + ":password")
	m.Secrets.Delete("profile:" + id + ":fernet")
	m.Profile.message = "Profile deleted"
	m.Profile.messageType = "success"
}

func (m *Model) testProfileConnection(id string) {
	profile := m.loadFullProfile(id)
	if profile == nil {
		m.Profile.message = "Failed to load profile"
		m.Profile.messageType = "error"
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := m.Migrator.TestConnection(ctx, profile)
	if err != nil {
		m.Profile.message = "Connection failed: " + err.Error()
		m.Profile.messageType = "error"
	} else {
		m.Profile.message = "Connection successful!"
		m.Profile.messageType = "success"
	}
}

func (m *Model) viewProfiles() string {
	switch m.Profile.state {
	case profileList:
		return m.viewProfileList()
	case profileAdd:
		return m.viewProfileForm("Add New Profile")
	case profileEdit:
		return m.viewProfileForm("Edit Profile")
	case profileDelete:
		return m.viewProfileDelete()
	}
	return ""
}

func (m *Model) viewProfileList() string {
	var s strings.Builder

	s.WriteString(TitleStyle.Render("📋 Connection Profiles"))
	s.WriteString("\n\n")

	if len(m.Profile.profiles) == 0 {
		s.WriteString(SubtleStyle.Render("No profiles yet. Press 'a' to add one."))
		s.WriteString("\n\n")
	} else {
		for i, p := range m.Profile.profiles {
			cursor := "  "
			if i == m.Profile.cursor {
				cursor = "▸ "
			}

			line := fmt.Sprintf("%s%s", cursor, p.Name)
			detail := fmt.Sprintf(" (%s/%s)", p.DBHost, p.DBName)

			if i == m.Profile.cursor {
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

	if m.Profile.message != "" {
		if m.Profile.messageType == "error" {
			s.WriteString(ErrorStyle.Render("✗ " + m.Profile.message))
		} else {
			s.WriteString(SuccessStyle.Render("✓ " + m.Profile.message))
		}
		s.WriteString("\n\n")
	}

	s.WriteString(SubtleStyle.Render("[a]dd  [e]dit  [d]elete  [t]est  [r]efresh  [q]back"))

	return s.String()
}

func (m *Model) viewProfileForm(title string) string {
	var s strings.Builder

	s.WriteString(TitleStyle.Render(title))
	s.WriteString("\n\n")

	labels := []string{"Name", "Host", "Port", "Database", "User", "Password", "Fernet Key"}

	for i, label := range labels {
		s.WriteString(fmt.Sprintf("%s:\n", label))
		s.WriteString(m.Profile.inputs[i].View())
		s.WriteString("\n\n")
	}

	if m.Profile.message != "" {
		if m.Profile.messageType == "error" {
			s.WriteString(ErrorStyle.Render("✗ " + m.Profile.message))
		} else {
			s.WriteString(SuccessStyle.Render("✓ " + m.Profile.message))
		}
		s.WriteString("\n\n")
	}

	s.WriteString(SubtleStyle.Render("[Tab] next  [Ctrl+S] save  [Ctrl+G] gen fernet  [Esc] cancel"))

	return s.String()
}

func (m *Model) viewProfileDelete() string {
	var s strings.Builder

	s.WriteString(TitleStyle.Render("⚠️  Delete Profile"))
	s.WriteString("\n\n")

	var profileName string
	for _, p := range m.Profile.profiles {
		if p.ID == m.Profile.deleteID {
			profileName = p.Name
			break
		}
	}

	s.WriteString(fmt.Sprintf("Are you sure you want to delete '%s'?\n\n", profileName))
	s.WriteString("This action cannot be undone.\n\n")
	s.WriteString(SubtleStyle.Render("[y]es  [n]o"))

	return s.String()
}
