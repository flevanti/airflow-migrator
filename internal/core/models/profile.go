package models

import (
	"fmt"
	"time"
)

// Profile represents a saved connection profile for an Airflow instance.
// It contains all the information needed to connect to an Airflow metadata DB
// and handle Fernet encryption/decryption.
type Profile struct {
	// Identity
	ID        string    `json:"id"`   // Unique identifier (UUID)
	Name      string    `json:"name"` // Human-readable name (e.g., "Dev Environment")
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Database connection
	DBHost     string `json:"db_host"`
	DBPort     int    `json:"db_port"`
	DBName     string `json:"db_name"` // Database name
	DBUser     string `json:"db_user"`
	DBPassword string `json:"db_password"` // Stored encrypted in SecretStore
	DBSSLMode  string `json:"db_ssl_mode"` // disable, require, verify-ca, verify-full

	// Fernet key for this Airflow instance
	// Used to decrypt passwords/extras from DB or encrypt when importing
	FernetKey string `json:"fernet_key"` // Stored encrypted in SecretStore

	// Optional settings
	ConnectionPrefix string `json:"connection_prefix"` // Prefix to add to conn_ids on import
}

// Default values
const (
	DefaultDBPort    = 5432
	DefaultDBSSLMode = "disable"
)

// NewProfile creates a new profile with default values
func NewProfile(name string) *Profile {
	now := time.Now().UTC()
	return &Profile{
		ID:        generateID(),
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
		DBPort:    DefaultDBPort,
		DBSSLMode: DefaultDBSSLMode,
	}
}

// generateID creates a simple unique ID
// In production, use github.com/google/uuid
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// Validate checks if the profile has all required fields
func (p *Profile) Validate() error {
	if p.ID == "" {
		return fmt.Errorf("profile ID is required")
	}
	if p.Name == "" {
		return fmt.Errorf("profile name is required")
	}
	if p.DBHost == "" {
		return fmt.Errorf("database host is required")
	}
	if p.DBPort <= 0 || p.DBPort > 65535 {
		return fmt.Errorf("invalid database port: %d", p.DBPort)
	}
	if p.DBName == "" {
		return fmt.Errorf("database name is required")
	}
	if p.DBUser == "" {
		return fmt.Errorf("database user is required")
	}
	if p.FernetKey == "" {
		return fmt.Errorf("fernet key is required")
	}
	return nil
}

// ConnectionString returns a PostgreSQL connection string
func (p *Profile) ConnectionString() string {
	return fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
		p.DBHost, p.DBPort, p.DBName, p.DBUser, p.DBPassword, p.DBSSLMode,
	)
}

// DSN returns a PostgreSQL DSN (Data Source Name) URL format
func (p *Profile) DSN() string {
	sslMode := p.DBSSLMode
	if sslMode == "" {
		sslMode = "disable"
	}
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		p.DBUser, p.DBPassword, p.DBHost, p.DBPort, p.DBName, sslMode,
	)
}

// Clone creates a deep copy of the profile
func (p *Profile) Clone() *Profile {
	return &Profile{
		ID:               p.ID,
		Name:             p.Name,
		CreatedAt:        p.CreatedAt,
		UpdatedAt:        p.UpdatedAt,
		DBHost:           p.DBHost,
		DBPort:           p.DBPort,
		DBName:           p.DBName,
		DBUser:           p.DBUser,
		DBPassword:       p.DBPassword,
		DBSSLMode:        p.DBSSLMode,
		FernetKey:        p.FernetKey,
		ConnectionPrefix: p.ConnectionPrefix,
	}
}

// Touch updates the UpdatedAt timestamp
func (p *Profile) Touch() {
	p.UpdatedAt = time.Now().UTC()
}

// SecretKeys returns the keys used to store secrets in the SecretStore
type ProfileSecretKeys struct {
	Password  string
	FernetKey string
}

// GetSecretKeys returns the SecretStore keys for this profile's secrets
func (p *Profile) GetSecretKeys() ProfileSecretKeys {
	return ProfileSecretKeys{
		Password:  fmt.Sprintf("profile:%s:password", p.ID),
		FernetKey: fmt.Sprintf("profile:%s:fernet", p.ID),
	}
}

// ProfileSummary is a lightweight version of Profile for listing
// Does not include sensitive fields
type ProfileSummary struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	DBHost    string    `json:"db_host"`
	DBPort    int       `json:"db_port"`
	DBName    string    `json:"db_name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Summary returns a ProfileSummary (safe for display/logging)
func (p *Profile) Summary() ProfileSummary {
	return ProfileSummary{
		ID:        p.ID,
		Name:      p.Name,
		DBHost:    p.DBHost,
		DBPort:    p.DBPort,
		DBName:    p.DBName,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
}
