// Package models defines the core data structures for the Airflow Connection Migrator.
package models

import (
	"fmt"
	"time"
)

// Connection represents an Airflow connection record.
// This mirrors the structure in Airflow's connection table.
type Connection struct {
	// Core identifiers
	ID          string `json:"id"`          // Unique identifier (conn_id in Airflow)
	ConnType    string `json:"conn_type"`   // Connection type (e.g., postgres, mysql, http)
	Description string `json:"description"` // Human-readable description

	// Connection details
	Host     string `json:"host"`
	Schema   string `json:"schema"`   // Database name or schema
	Login    string `json:"login"`    // Username
	Password string `json:"password"` // Password (encrypted in Airflow DB)
	Port     int    `json:"port"`
	Extra    string `json:"extra"` // JSON string with additional parameters

	// Metadata
	IsEncrypted bool `json:"is_encrypted"` // Whether password/extra are encrypted
	IsTested    bool `json:"is_tested"`    // Whether connection was tested successfully
}

// ConnectionType constants for common Airflow connection types
const (
	ConnTypePostgres = "postgres"
	ConnTypeMySQL    = "mysql"
	ConnTypeMSSQL    = "mssql"
	ConnTypeOracle   = "oracle"
	ConnTypeHTTP     = "http"
	ConnTypeHTTPS    = "https"
	ConnTypeSSH      = "ssh"
	ConnTypeFTP      = "ftp"
	ConnTypeSFTP     = "sftp"
	ConnTypeS3       = "s3"
	ConnTypeAWS      = "aws"
	ConnTypeGCP      = "google_cloud_platform"
	ConnTypeAzure    = "azure"
	ConnTypeSlack    = "slack"
	ConnTypeEmail    = "email"
	ConnTypeSMTP     = "smtp"
	ConnTypeGeneric  = "generic"
)

// Validate checks if the connection has required fields
func (c *Connection) Validate() error {
	if c.ID == "" {
		return fmt.Errorf("connection ID is required")
	}
	if c.ConnType == "" {
		return fmt.Errorf("connection type is required")
	}
	return nil
}

// String returns a human-readable representation (without sensitive data)
func (c *Connection) String() string {
	return fmt.Sprintf("Connection{ID: %s, Type: %s, Host: %s, Port: %d}",
		c.ID, c.ConnType, c.Host, c.Port)
}

// Clone creates a deep copy of the connection
func (c *Connection) Clone() *Connection {
	return &Connection{
		ID:          c.ID,
		ConnType:    c.ConnType,
		Description: c.Description,
		Host:        c.Host,
		Schema:      c.Schema,
		Login:       c.Login,
		Password:    c.Password,
		Port:        c.Port,
		Extra:       c.Extra,
		IsEncrypted: c.IsEncrypted,
		IsTested:    c.IsTested,
	}
}

// ExportRecord represents a connection in the export CSV format.
// This is the intermediate format used when exporting/importing.
type ExportRecord struct {
	ConnID      string `json:"conn_id"`
	ConnType    string `json:"conn_type"`
	Description string `json:"description"`
	Host        string `json:"host"`
	Schema      string `json:"schema"`
	Login       string `json:"login"`
	Password    string `json:"password"` // Encrypted with file Fernet key
	Port        int    `json:"port"`
	Extra       string `json:"extra"`       // Encrypted with file Fernet key
	ExportedAt  string `json:"exported_at"` // ISO 8601 timestamp
}

// ToExportRecord converts a Connection to an ExportRecord
func (c *Connection) ToExportRecord() *ExportRecord {
	return &ExportRecord{
		ConnID:      c.ID,
		ConnType:    c.ConnType,
		Description: c.Description,
		Host:        c.Host,
		Schema:      c.Schema,
		Login:       c.Login,
		Password:    c.Password,
		Port:        c.Port,
		Extra:       c.Extra,
		ExportedAt:  time.Now().UTC().Format(time.RFC3339),
	}
}

// ToConnection converts an ExportRecord back to a Connection
func (r *ExportRecord) ToConnection() *Connection {
	return &Connection{
		ID:          r.ConnID,
		ConnType:    r.ConnType,
		Description: r.Description,
		Host:        r.Host,
		Schema:      r.Schema,
		Login:       r.Login,
		Password:    r.Password,
		Port:        r.Port,
		Extra:       r.Extra,
		IsEncrypted: false, // Will be re-encrypted during import
	}
}
