package api

import (
	"encoding/json"
	"net/http"

	"github.com/flevanti/airflow-migrator/internal/core"
	"github.com/flevanti/airflow-migrator/internal/core/models"
	"github.com/flevanti/airflow-migrator/internal/secrets"
)

// Server is the HTTP API server.
type Server struct {
	migrator  *core.Migrator
	secrets   *secrets.Store
	mux       *http.ServeMux
	configDir string
}

// NewServer creates a new HTTP server.
func NewServer(migrator *core.Migrator, secrets *secrets.Store, configDir string) *Server {
	s := &Server{
		migrator:  migrator,
		secrets:   secrets,
		mux:       http.NewServeMux(),
		configDir: configDir,
	}
	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	// Health check
	s.mux.HandleFunc("GET /health", s.handleHealth)

	// Web pages and HTMX endpoints
	s.SetupWebRoutes()

	// JSON API - Connections
	s.mux.HandleFunc("POST /api/connections/list", s.handleListConnections)
	s.mux.HandleFunc("POST /api/connections/export", s.handleExport)
	s.mux.HandleFunc("POST /api/connections/import", s.handleImport)
	s.mux.HandleFunc("POST /api/connections/test", s.handleTestConnection)

	// Fernet
	s.mux.HandleFunc("GET /api/fernet/generate", s.handleGenerateFernetKey)
	s.mux.HandleFunc("POST /api/fernet/validate", s.handleValidateFernetKey)

	// Profiles
	s.mux.HandleFunc("GET /api/profiles", s.handleListProfiles)
	s.mux.HandleFunc("POST /api/profiles", s.handleSaveProfile)
	s.mux.HandleFunc("DELETE /api/profiles/{id}", s.handleDeleteProfile)
}

// Start starts the HTTP server.
func (s *Server) Start(addr string) error {
	return http.ListenAndServe(addr, s.mux)
}

// Health check
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// List connections from a profile
func (s *Server) handleListConnections(w http.ResponseWriter, r *http.Request) {
	var req models.ListConnectionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, "invalid request", http.StatusBadRequest)
		return
	}

	// Load secrets into profile
	if err := s.loadProfileSecrets(req.Profile); err != nil {
		httpError(w, err.Error(), http.StatusBadRequest)
		return
	}

	connections, err := s.migrator.ListConnections(r.Context(), req.Profile)
	if err != nil {
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(models.ListConnectionsResult{
		Success:     true,
		Connections: connections,
		Count:       len(connections),
	})
}

// Export connections
func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	var req models.ExportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, "invalid request", http.StatusBadRequest)
		return
	}

	if err := s.loadProfileSecrets(req.SourceProfile); err != nil {
		httpError(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := s.migrator.Export(r.Context(), req)
	if err != nil {
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(result)
}

// Import connections
func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	var req models.ImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, "invalid request", http.StatusBadRequest)
		return
	}

	if err := s.loadProfileSecrets(req.TargetProfile); err != nil {
		httpError(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := s.migrator.Import(r.Context(), req)
	if err != nil {
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(result)
}

// Test database connection
func (s *Server) handleTestConnection(w http.ResponseWriter, r *http.Request) {
	var req models.TestConnectionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, "invalid request", http.StatusBadRequest)
		return
	}

	if err := s.loadProfileSecrets(req.Profile); err != nil {
		httpError(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := s.migrator.TestConnection(r.Context(), req.Profile)

	result := models.TestConnectionResult{Success: err == nil}
	if err != nil {
		result.Error = err.Error()
		result.Message = "Connection failed"
	} else {
		result.Message = "Connection successful"
	}

	json.NewEncoder(w).Encode(result)
}

// Generate Fernet key
func (s *Server) handleGenerateFernetKey(w http.ResponseWriter, r *http.Request) {
	key, err := s.migrator.GenerateFernetKey()
	if err != nil {
		httpError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(models.GenerateFernetKeyResult{Key: key})
}

// Validate Fernet key
func (s *Server) handleValidateFernetKey(w http.ResponseWriter, r *http.Request) {
	var req models.ValidateFernetKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, "invalid request", http.StatusBadRequest)
		return
	}

	valid := s.migrator.ValidateFernetKey(req.Key)

	result := models.ValidateFernetKeyResult{Valid: valid}
	if valid {
		result.Message = "Valid Fernet key"
	} else {
		result.Message = "Invalid Fernet key"
	}

	json.NewEncoder(w).Encode(result)
}

// List saved profiles
func (s *Server) handleListProfiles(w http.ResponseWriter, r *http.Request) {
	// Get all profile keys from secrets
	keys := s.secrets.List()

	profileIDs := make(map[string]bool)
	for _, key := range keys {
		// Keys are like "profile:123:password", "profile:123:fernet"
		if len(key) > 8 && key[:8] == "profile:" {
			parts := splitKey(key)
			if len(parts) >= 2 {
				profileIDs[parts[1]] = true
			}
		}
	}

	// Load profile metadata
	var profiles []models.ProfileSummary
	for id := range profileIDs {
		metaKey := "profile:" + id + ":meta"
		if metaJSON, err := s.secrets.Get(metaKey); err == nil {
			var summary models.ProfileSummary
			if json.Unmarshal([]byte(metaJSON), &summary) == nil {
				profiles = append(profiles, summary)
			}
		}
	}

	json.NewEncoder(w).Encode(profiles)
}

// Save profile
func (s *Server) handleSaveProfile(w http.ResponseWriter, r *http.Request) {
	var profile models.Profile
	if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
		httpError(w, "invalid request", http.StatusBadRequest)
		return
	}

	keys := profile.GetSecretKeys()

	// Store password
	if profile.DBPassword != "" {
		if err := s.secrets.Set(keys.Password, profile.DBPassword); err != nil {
			httpError(w, "failed to save password", http.StatusInternalServerError)
			return
		}
	}

	// Store Fernet key
	if profile.FernetKey != "" {
		if err := s.secrets.Set(keys.FernetKey, profile.FernetKey); err != nil {
			httpError(w, "failed to save fernet key", http.StatusInternalServerError)
			return
		}
	}

	// Store metadata (non-sensitive)
	profile.Touch()
	metaKey := "profile:" + profile.ID + ":meta"
	metaJSON, _ := json.Marshal(profile.Summary())
	if err := s.secrets.Set(metaKey, string(metaJSON)); err != nil {
		httpError(w, "failed to save profile metadata", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "saved", "id": profile.ID})
}

// Delete profile
func (s *Server) handleDeleteProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		httpError(w, "profile ID required", http.StatusBadRequest)
		return
	}

	// Delete all profile keys
	keys := []string{
		"profile:" + id + ":password",
		"profile:" + id + ":fernet",
		"profile:" + id + ":meta",
	}

	for _, key := range keys {
		s.secrets.Delete(key) // Ignore errors for non-existent keys
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

// Helper: load secrets into profile
func (s *Server) loadProfileSecrets(profile *models.Profile) error {
	if profile == nil {
		return nil
	}

	keys := profile.GetSecretKeys()

	// Load password if not provided
	if profile.DBPassword == "" {
		if pw, err := s.secrets.Get(keys.Password); err == nil {
			profile.DBPassword = pw
		}
	}

	// Load Fernet key if not provided
	if profile.FernetKey == "" {
		if fk, err := s.secrets.Get(keys.FernetKey); err == nil {
			profile.FernetKey = fk
		}
	}

	return nil
}

func httpError(w http.ResponseWriter, message string, code int) {
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func splitKey(key string) []string {
	var parts []string
	current := ""
	for _, c := range key {
		if c == ':' {
			parts = append(parts, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	parts = append(parts, current)
	return parts
}
