package api

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/flevanti/airflow-migrator/internal/app"
	"github.com/flevanti/airflow-migrator/internal/core/models"
	"github.com/flevanti/airflow-migrator/internal/core/services"
	"github.com/flevanti/airflow-migrator/web"
)

var templates *template.Template

func init() {
	templates = template.Must(template.ParseFS(web.TemplateFS, "templates/*.html"))
}

// SetupWebRoutes adds HTML page routes
func (s *Server) SetupWebRoutes() {
	// Pages
	s.mux.HandleFunc("GET /{$}", s.handleHomePage)
	s.mux.HandleFunc("GET /profiles", s.handleProfilesPage)
	s.mux.HandleFunc("GET /export", s.handleExportPage)
	s.mux.HandleFunc("GET /import", s.handleImportPage)
	s.mux.HandleFunc("GET /about", s.handleAboutPage)

	// HTMX endpoints (return HTML fragments)
	s.mux.HandleFunc("GET /htmx/fernet/generate", s.htmxGenerateFernetKey)
	s.mux.HandleFunc("POST /htmx/fernet/validate", s.htmxValidateFernet)
	s.mux.HandleFunc("GET /htmx/profiles/list", s.htmxListProfiles)
	s.mux.HandleFunc("POST /htmx/profiles/save", s.htmxSaveProfile)
	s.mux.HandleFunc("GET /htmx/profiles/{id}", s.htmxGetProfile)
	s.mux.HandleFunc("POST /htmx/profiles/test", s.htmxTestProfile)
	s.mux.HandleFunc("DELETE /htmx/profiles/{id}", s.htmxDeleteProfile)
	s.mux.HandleFunc("GET /htmx/connections/list", s.htmxListConnections)
	s.mux.HandleFunc("POST /htmx/export", s.htmxExport)
	s.mux.HandleFunc("POST /htmx/import", s.htmxImport)
	s.mux.HandleFunc("POST /htmx/import/preview", s.htmxImportPreview)
	s.mux.HandleFunc("GET /download/{filename}", s.handleDownload)
}

func (s *Server) renderPage(w http.ResponseWriter, page string, data any) {
	w.Header().Set("Content-Type", "text/html")
	err := templates.ExecuteTemplate(w, page+".html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) renderPartial(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html")
	err := templates.ExecuteTemplate(w, name, data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// Page handlers
func (s *Server) handleHomePage(w http.ResponseWriter, r *http.Request) {
	s.renderPage(w, "home", map[string]any{"Page": "home", "Title": "Home"})
}

func (s *Server) handleProfilesPage(w http.ResponseWriter, r *http.Request) {
	s.renderPage(w, "profiles", map[string]any{"Page": "profiles", "Title": "Profiles"})
}

func (s *Server) handleExportPage(w http.ResponseWriter, r *http.Request) {
	profiles := s.getProfileSummaries()
	s.renderPage(w, "export", map[string]any{"Page": "export", "Title": "Export", "Profiles": profiles})
}

func (s *Server) handleImportPage(w http.ResponseWriter, r *http.Request) {
	profiles := s.getProfileSummaries()
	s.renderPage(w, "import", map[string]any{"Page": "import", "Title": "Import", "Profiles": profiles})
}

func (s *Server) handleAboutPage(w http.ResponseWriter, r *http.Request) {
	s.renderPage(w, "about", map[string]any{
		"Page":            "about",
		"Title":           "About",
		"AppName":         app.Name,
		"Version":         app.Version,
		"Build":           app.Build,
		"License":         app.License,
		"GitHub":          app.GitHub,
		"GitHubPR":        app.GitHubPR,
		"GitHubIssues":    app.GitHubIssues,
		"DevName":         app.DevName,
		"DevEmail":        app.DevEmail,
		"Year":            app.CurrentYear(),
		"ConfigDir":       s.configDir,
		"TempDir":         os.TempDir(),
		"CredentialsFile": filepath.Join(s.configDir, "credentials.enc"),
	})
}

// HTMX handlers
func (s *Server) htmxGenerateFernetKey(w http.ResponseWriter, r *http.Request) {
	key, _ := s.migrator.GenerateFernetKey()
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(key))
}

func (s *Server) htmxValidateFernet(w http.ResponseWriter, r *http.Request) {
	key := r.FormValue("key")
	if s.migrator.ValidateFernetKey(key) {
		s.renderPartial(w, "validate-valid", nil)
	} else {
		s.renderPartial(w, "validate-invalid", nil)
	}
}

func (s *Server) htmxListProfiles(w http.ResponseWriter, r *http.Request) {
	profiles := s.getProfileSummaries()
	s.renderPartial(w, "profiles-list", map[string]any{"Profiles": profiles})
}

func (s *Server) htmxSaveProfile(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	port, _ := strconv.Atoi(r.FormValue("db_port"))

	profile := models.NewProfile(r.FormValue("name"))
	profile.DBHost = r.FormValue("db_host")
	profile.DBPort = port
	profile.DBName = r.FormValue("db_name")
	profile.DBUser = r.FormValue("db_user")
	profile.DBSSLMode = r.FormValue("db_ssl_mode")

	// Check if editing existing profile
	existingID := r.FormValue("id")
	if existingID != "" {
		profile.ID = existingID
		// Load existing secrets if not provided
		existingProfile := s.loadProfile(existingID)
		if existingProfile != nil {
			if r.FormValue("db_password") == "" {
				profile.DBPassword = existingProfile.DBPassword
			} else {
				profile.DBPassword = r.FormValue("db_password")
			}
			if r.FormValue("fernet_key") == "" {
				profile.FernetKey = existingProfile.FernetKey
			} else {
				profile.FernetKey = r.FormValue("fernet_key")
			}
		}
	} else {
		profile.DBPassword = r.FormValue("db_password")
		profile.FernetKey = r.FormValue("fernet_key")
	}

	// Save secrets
	keys := profile.GetSecretKeys()
	s.secrets.Set(keys.Password, profile.DBPassword)
	s.secrets.Set(keys.FernetKey, profile.FernetKey)

	// Save metadata
	metaKey := "profile:" + profile.ID + ":meta"
	s.secrets.Set(metaKey, profileToJSON(profile))

	// Return updated list
	s.htmxListProfiles(w, r)
}

func (s *Server) htmxGetProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	profile := s.loadProfile(id)
	if profile == nil {
		http.Error(w, "Profile not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"id":      profile.ID,
		"name":    profile.Name,
		"db_host": profile.DBHost,
		"db_port": profile.DBPort,
		"db_name": profile.DBName,
		"db_user": profile.DBUser,
	})
}

func (s *Server) htmxTestProfile(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	profileID := r.FormValue("id")

	profile := s.loadProfile(profileID)
	if profile == nil {
		s.renderPartial(w, "test-fail", nil)
		return
	}

	err := s.migrator.TestConnection(r.Context(), profile)
	if err != nil {
		s.renderPartial(w, "test-fail", nil)
	} else {
		s.renderPartial(w, "test-success", nil)
	}
}

func (s *Server) htmxDeleteProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Delete all keys
	s.secrets.Delete("profile:" + id + ":password")
	s.secrets.Delete("profile:" + id + ":fernet")
	s.secrets.Delete("profile:" + id + ":meta")

	s.htmxListProfiles(w, r)
}

func (s *Server) htmxListConnections(w http.ResponseWriter, r *http.Request) {
	profileID := r.URL.Query().Get("profile_id")
	if profileID == "" {
		s.renderPartial(w, "connections-list", map[string]any{"Connections": nil})
		return
	}

	profile := s.loadProfile(profileID)
	if profile == nil {
		s.renderPartial(w, "connections-list", map[string]any{"Connections": nil})
		return
	}

	connections, err := s.migrator.ListConnections(r.Context(), profile)
	if err != nil {
		s.renderPartial(w, "connections-list", map[string]any{"Connections": nil, "Error": err.Error()})
		return
	}

	s.renderPartial(w, "connections-list", map[string]any{"Connections": connections})
}

func (s *Server) htmxExport(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	profileID := r.FormValue("profile_id")
	profile := s.loadProfile(profileID)
	if profile == nil {
		s.renderPartial(w, "export-result", &models.ExportResult{Error: "Profile not found"})
		return
	}

	// Generate filename with profile name and timestamp
	timestamp := time.Now().Format("2006-01-02_150405")
	safeName := strings.ReplaceAll(profile.Name, " ", "_")
	filename := fmt.Sprintf("airflow_%s_%s.csv", safeName, timestamp)
	tempPath := filepath.Join(os.TempDir(), filename)

	req := models.ExportRequest{
		SourceProfile:     profile,
		OutputPath:        tempPath,
		FileEncryptionKey: r.FormValue("file_key"),
		ConnectionIDs:     r.Form["connection_ids"],
	}

	result, _ := s.migrator.Export(r.Context(), req)

	if result.Success {
		// Store file info for download
		result.OutputPath = filename
		result.DownloadURL = "/download/" + filepath.Base(tempPath)
	}

	s.renderPartial(w, "export-result", result)
}

func (s *Server) htmxImportPreview(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form (max 10MB)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	fileKey := r.FormValue("file_key")
	if fileKey == "" {
		http.Error(w, "Decryption key is required", http.StatusBadRequest)
		return
	}

	// Get uploaded file
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "No file uploaded", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Save to temp file
	tempFile := filepath.Join(os.TempDir(), "airflow-preview-"+header.Filename)
	out, err := os.Create(tempFile)
	if err != nil {
		http.Error(w, "Failed to create temp file", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tempFile)
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}
	out.Close()

	// Create Fernet to decrypt
	fernet, err := services.NewFernet(fileKey)
	if err != nil {
		http.Error(w, "Invalid decryption key", http.StatusBadRequest)
		return
	}

	// Read and decrypt CSV
	records, err := services.ReadEncryptedCSV(tempFile, fernet)
	if err != nil {
		http.Error(w, "Failed to decrypt file: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Render connections list
	s.renderPartial(w, "import-connections-list", map[string]any{"Records": records})
}

func (s *Server) htmxImport(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form (max 10MB)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		s.renderPartial(w, "import-result", &models.ImportResult{Error: "Failed to parse form: " + err.Error()})
		return
	}

	profileID := r.FormValue("profile_id")
	profile := s.loadProfile(profileID)
	if profile == nil {
		s.renderPartial(w, "import-result", &models.ImportResult{Error: "Profile not found"})
		return
	}

	// Get uploaded file
	file, header, err := r.FormFile("file")
	if err != nil {
		s.renderPartial(w, "import-result", &models.ImportResult{Error: "No file uploaded"})
		return
	}
	defer file.Close()

	// Save to temp file
	tempFile := filepath.Join(os.TempDir(), "airflow-import-"+header.Filename)
	out, err := os.Create(tempFile)
	if err != nil {
		s.renderPartial(w, "import-result", &models.ImportResult{Error: "Failed to create temp file"})
		return
	}
	defer os.Remove(tempFile)
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		s.renderPartial(w, "import-result", &models.ImportResult{Error: "Failed to save file"})
		return
	}
	out.Close()

	collision := models.CollisionStrategy(r.FormValue("collision"))

	req := models.ImportRequest{
		TargetProfile:     profile,
		InputPath:         tempFile,
		FileDecryptionKey: r.FormValue("file_key"),
		CollisionStrategy: collision,
		ConnectionPrefix:  r.FormValue("prefix"),
	}

	result, _ := s.migrator.Import(r.Context(), req)
	s.renderPartial(w, "import-result", result)
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	filename := r.PathValue("filename")
	if filename == "" {
		http.Error(w, "Filename required", http.StatusBadRequest)
		return
	}

	// Security: only allow files from temp directory
	tempPath := filepath.Join(os.TempDir(), filename)

	// Verify the file exists and is in temp directory
	absPath, err := filepath.Abs(tempPath)
	if err != nil || !strings.HasPrefix(absPath, os.TempDir()) {
		http.Error(w, "Invalid file", http.StatusBadRequest)
		return
	}

	file, err := os.Open(tempPath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	// Set headers for download
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Content-Type", "text/csv")

	// Copy file to response
	io.Copy(w, file)

	// Delete file after download
	file.Close()
	os.Remove(tempPath)
}

// Helpers
func (s *Server) getProfileSummaries() []models.ProfileSummary {
	var profiles []models.ProfileSummary

	for _, key := range s.secrets.List() {
		if len(key) > 8 && key[:8] == "profile:" && len(key) > 5 && key[len(key)-5:] == ":meta" {
			if metaJSON, err := s.secrets.Get(key); err == nil {
				if summary := jsonToProfileSummary(metaJSON); summary != nil {
					profiles = append(profiles, *summary)
				}
			}
		}
	}

	// Sort alphabetically by name
	sort.Slice(profiles, func(i, j int) bool {
		return strings.ToLower(profiles[i].Name) < strings.ToLower(profiles[j].Name)
	})

	return profiles
}

func (s *Server) loadProfile(id string) *models.Profile {
	metaKey := "profile:" + id + ":meta"
	metaJSON, err := s.secrets.Get(metaKey)
	if err != nil {
		return nil
	}

	// Parse stored profile data
	profile := &models.Profile{}
	if err := json.Unmarshal([]byte(metaJSON), profile); err != nil {
		return nil
	}

	// Load secrets
	keys := profile.GetSecretKeys()
	if pw, err := s.secrets.Get(keys.Password); err == nil {
		profile.DBPassword = pw
	}
	if fk, err := s.secrets.Get(keys.FernetKey); err == nil {
		profile.FernetKey = fk
	}

	// Set default SSL mode if not stored
	if profile.DBSSLMode == "" {
		profile.DBSSLMode = "disable"
	}

	return profile
}

func profileToJSON(p *models.Profile) string {
	// Store all non-secret fields
	data := map[string]any{
		"id":      p.ID,
		"name":    p.Name,
		"db_host": p.DBHost,
		"db_port": p.DBPort,
		"db_name": p.DBName,
		"db_user": p.DBUser,
	}
	b, _ := json.Marshal(data)
	return string(b)
}

func jsonToProfileSummary(jsonStr string) *models.ProfileSummary {
	var data map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return nil
	}

	summary := &models.ProfileSummary{}
	if v, ok := data["id"].(string); ok {
		summary.ID = v
	}
	if v, ok := data["name"].(string); ok {
		summary.Name = v
	}
	if v, ok := data["db_host"].(string); ok {
		summary.DBHost = v
	}
	if v, ok := data["db_name"].(string); ok {
		summary.DBName = v
	}

	if summary.ID == "" {
		return nil
	}
	return summary
}
