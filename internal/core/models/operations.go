package models

// CollisionStrategy defines how to handle existing connections during import
type CollisionStrategy string

const (
	// CollisionStop stops the import if any connection already exists
	CollisionStop CollisionStrategy = "stop"

	// CollisionSkip skips connections that already exist, imports the rest
	CollisionSkip CollisionStrategy = "skip"

	// CollisionOverwrite overwrites existing connections
	CollisionOverwrite CollisionStrategy = "overwrite"
)

// ExportRequest contains parameters for an export operation
type ExportRequest struct {
	// Source profile to export from
	SourceProfile *Profile `json:"source_profile"`

	// Connections to export (if empty, exports all)
	ConnectionIDs []string `json:"connection_ids,omitempty"`

	// Output file path
	OutputPath string `json:"output_path"`

	// Fernet key for encrypting the export file
	// If empty, a new key will be generated
	FileEncryptionKey string `json:"file_encryption_key,omitempty"`
}

// ExportResult contains the result of an export operation
type ExportResult struct {
	Success           bool     `json:"success"`
	OutputPath        string   `json:"output_path"`
	ConnectionCount   int      `json:"connection_count"`
	ExportedIDs       []string `json:"exported_ids"`
	FileEncryptionKey string   `json:"file_encryption_key"` // The key used (generated or provided)
	Error             string   `json:"error,omitempty"`
	DownloadURL       string   `json:"download_url,omitempty"`
}

// ImportRequest contains parameters for an import operation
type ImportRequest struct {
	// Target profile to import into
	TargetProfile *Profile `json:"target_profile"`

	// Input file path
	InputPath string `json:"input_path"`

	// Fernet key for decrypting the import file
	FileDecryptionKey string `json:"file_decryption_key"`

	// How to handle existing connections
	CollisionStrategy CollisionStrategy `json:"collision_strategy"`

	// Optional prefix to add to connection IDs
	ConnectionPrefix string `json:"connection_prefix,omitempty"`

	// Specific connections to import (if empty, imports all)
	ConnectionIDs []string `json:"connection_ids,omitempty"`
}

// ImportResult contains the result of an import operation
type ImportResult struct {
	Success          bool     `json:"success"`
	ImportedCount    int      `json:"imported_count"`
	SkippedCount     int      `json:"skipped_count"`
	OverwrittenCount int      `json:"overwritten_count"`
	ImportedIDs      []string `json:"imported_ids"`
	SkippedIDs       []string `json:"skipped_ids,omitempty"`
	OverwrittenIDs   []string `json:"overwritten_ids,omitempty"`
	Error            string   `json:"error,omitempty"`
}

// TestConnectionRequest contains parameters for testing a database connection
type TestConnectionRequest struct {
	Profile *Profile `json:"profile"`
}

// TestConnectionResult contains the result of a connection test
type TestConnectionResult struct {
	Success      bool   `json:"success"`
	Message      string `json:"message"`
	ResponseTime int64  `json:"response_time_ms"` // Time in milliseconds
	Error        string `json:"error,omitempty"`
}

// ValidateFernetKeyRequest contains a Fernet key to validate
type ValidateFernetKeyRequest struct {
	Key string `json:"key"`
}

// ValidateFernetKeyResult contains the validation result
type ValidateFernetKeyResult struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message"`
}

// ListConnectionsRequest contains parameters for listing connections
type ListConnectionsRequest struct {
	Profile *Profile `json:"profile"`

	// Optional filter by connection type
	ConnType string `json:"conn_type,omitempty"`

	// Optional search term (searches in ID and description)
	Search string `json:"search,omitempty"`
}

// ListConnectionsResult contains the list of connections
type ListConnectionsResult struct {
	Success     bool          `json:"success"`
	Connections []*Connection `json:"connections"`
	Count       int           `json:"count"`
	Error       string        `json:"error,omitempty"`
}

// GenerateFernetKeyResult contains a newly generated Fernet key
type GenerateFernetKeyResult struct {
	Key string `json:"key"`
}
