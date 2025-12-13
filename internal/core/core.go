// Package core provides the main business logic for the Airflow Connection Migrator.
package core

import (
	"context"
	"fmt"

	"github.com/flevanti/airflow-migrator/internal/core/models"
	"github.com/flevanti/airflow-migrator/internal/core/services"
)

// Migrator is the main API for the Airflow Connection Migrator.
// Both HTTP and TUI frontends use this same interface.
type Migrator struct{}

// New creates a new Migrator instance.
func New() *Migrator {
	return &Migrator{}
}

// Export exports connections from a source Airflow database to an encrypted CSV file.
func (m *Migrator) Export(ctx context.Context, req models.ExportRequest) (*models.ExportResult, error) {
	result := &models.ExportResult{OutputPath: req.OutputPath}

	// Validate request
	if err := req.SourceProfile.Validate(); err != nil {
		result.Error = err.Error()
		return result, nil
	}

	// Connect to source database
	db, err := services.NewDatabase(req.SourceProfile)
	if err != nil {
		result.Error = fmt.Sprintf("failed to connect to database: %v", err)
		return result, nil
	}
	defer db.Close()

	// Get source Fernet for decryption
	sourceFernet, err := services.NewFernet(req.SourceProfile.FernetKey)
	if err != nil {
		result.Error = fmt.Sprintf("invalid source fernet key: %v", err)
		return result, nil
	}

	// Get or generate file encryption key
	fileKey := req.FileEncryptionKey
	if fileKey == "" {
		fileKey, err = services.GenerateKey()
		if err != nil {
			result.Error = fmt.Sprintf("failed to generate file key: %v", err)
			return result, nil
		}
	}
	result.FileEncryptionKey = fileKey

	fileFernet, err := services.NewFernet(fileKey)
	if err != nil {
		result.Error = fmt.Sprintf("invalid file encryption key: %v", err)
		return result, nil
	}

	// List connections
	connections, err := db.ListConnections(ctx)
	if err != nil {
		result.Error = fmt.Sprintf("failed to list connections: %v", err)
		return result, nil
	}

	// Filter if specific IDs requested
	if len(req.ConnectionIDs) > 0 {
		idSet := make(map[string]bool)
		for _, id := range req.ConnectionIDs {
			idSet[id] = true
		}
		var filtered []*models.Connection
		for _, conn := range connections {
			if idSet[conn.ID] {
				filtered = append(filtered, conn)
			}
		}
		connections = filtered
	}

	// Process connections: decrypt password/extra with source key
	var records []*models.ExportRecord
	for _, conn := range connections {
		// Decrypt password if encrypted
		if conn.Password != "" {
			decrypted, err := sourceFernet.DecryptString(conn.Password)
			if err == nil {
				conn.Password = decrypted
			}
			// If decryption fails, assume plaintext
		}

		// Decrypt extra if encrypted
		if conn.Extra != "" {
			decrypted, err := sourceFernet.DecryptString(conn.Extra)
			if err == nil {
				conn.Extra = decrypted
			}
		}

		// Store decrypted values - will be encrypted as blob by WriteEncryptedCSV
		records = append(records, conn.ToExportRecord())
		result.ExportedIDs = append(result.ExportedIDs, conn.ID)
	}

	// Write encrypted CSV (entire connection blob encrypted with file key)
	if err := services.WriteEncryptedCSV(req.OutputPath, records, fileFernet); err != nil {
		result.Error = fmt.Sprintf("failed to write CSV: %v", err)
		return result, nil
	}

	result.Success = true
	result.ConnectionCount = len(records)
	return result, nil
}

// Import imports connections from an encrypted CSV file to a target Airflow database.
func (m *Migrator) Import(ctx context.Context, req models.ImportRequest) (*models.ImportResult, error) {
	result := &models.ImportResult{}

	// Validate request
	if err := req.TargetProfile.Validate(); err != nil {
		result.Error = err.Error()
		return result, nil
	}

	// Get file Fernet for decryption
	fileFernet, err := services.NewFernet(req.FileDecryptionKey)
	if err != nil {
		result.Error = fmt.Sprintf("invalid file decryption key: %v", err)
		return result, nil
	}

	// Read and decrypt CSV
	records, err := services.ReadEncryptedCSV(req.InputPath, fileFernet)
	if err != nil {
		result.Error = fmt.Sprintf("failed to read CSV: %v", err)
		return result, nil
	}

	if len(records) == 0 {
		result.Success = true
		return result, nil
	}

	// Connect to target database
	db, err := services.NewDatabase(req.TargetProfile)
	if err != nil {
		result.Error = fmt.Sprintf("failed to connect to database: %v", err)
		return result, nil
	}
	defer db.Close()

	// Get target Fernet for encryption
	targetFernet, err := services.NewFernet(req.TargetProfile.FernetKey)
	if err != nil {
		result.Error = fmt.Sprintf("invalid target fernet key: %v", err)
		return result, nil
	}

	// Filter if specific IDs requested
	if len(req.ConnectionIDs) > 0 {
		idSet := make(map[string]bool)
		for _, id := range req.ConnectionIDs {
			idSet[id] = true
		}
		var filtered []*models.ExportRecord
		for _, r := range records {
			if idSet[r.ConnID] {
				filtered = append(filtered, r)
			}
		}
		records = filtered
	}

	// Build list of IDs to check
	var idsToCheck []string
	for _, r := range records {
		connID := r.ConnID
		if req.ConnectionPrefix != "" {
			connID = req.ConnectionPrefix + connID
		}
		idsToCheck = append(idsToCheck, connID)
	}

	// Check for existing connections
	existingIDs, err := db.GetExistingConnectionIDs(ctx, idsToCheck)
	if err != nil {
		result.Error = fmt.Sprintf("failed to check existing connections: %v", err)
		return result, nil
	}
	existingSet := make(map[string]bool)
	for _, id := range existingIDs {
		existingSet[id] = true
	}

	// Handle collision strategy
	if req.CollisionStrategy == models.CollisionStop && len(existingIDs) > 0 {
		result.Error = fmt.Sprintf("connections already exist: %v", existingIDs)
		return result, nil
	}

	// Process and import
	for _, record := range records {
		conn := record.ToConnection()

		// Apply prefix
		if req.ConnectionPrefix != "" {
			conn.ID = req.ConnectionPrefix + conn.ID
		}

		// Check if exists
		exists := existingSet[conn.ID]

		if exists {
			switch req.CollisionStrategy {
			case models.CollisionSkip:
				result.SkippedIDs = append(result.SkippedIDs, conn.ID)
				result.SkippedCount++
				continue
			case models.CollisionOverwrite:
				// Will update below
			}
		}

		// Re-encrypt password and extra with target Fernet key
		if conn.Password != "" {
			encrypted, _ := targetFernet.EncryptString(conn.Password)
			conn.Password = encrypted
		}
		if conn.Extra != "" {
			encrypted, _ := targetFernet.EncryptString(conn.Extra)
			conn.Extra = encrypted
		}

		// Insert or update
		if exists {
			if err := db.UpdateConnection(ctx, conn); err != nil {
				result.Error = fmt.Sprintf("failed to update %s: %v", conn.ID, err)
				return result, nil
			}
			result.OverwrittenIDs = append(result.OverwrittenIDs, conn.ID)
			result.OverwrittenCount++
		} else {
			if err := db.InsertConnection(ctx, conn); err != nil {
				result.Error = fmt.Sprintf("failed to insert %s: %v", conn.ID, err)
				return result, nil
			}
			result.ImportedIDs = append(result.ImportedIDs, conn.ID)
			result.ImportedCount++
		}
	}

	result.Success = true
	return result, nil
}

// ListConnections lists all connections from an Airflow database.
func (m *Migrator) ListConnections(ctx context.Context, profile *models.Profile) ([]*models.Connection, error) {
	db, err := services.NewDatabase(profile)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	return db.ListConnections(ctx)
}

// TestConnection tests the database connection.
func (m *Migrator) TestConnection(ctx context.Context, profile *models.Profile) error {
	db, err := services.NewDatabase(profile)
	if err != nil {
		return err
	}
	defer db.Close()

	return db.TestConnection(ctx)
}

// ValidateFernetKey validates a Fernet key.
func (m *Migrator) ValidateFernetKey(key string) bool {
	return services.ValidateKey(key)
}

// GenerateFernetKey generates a new Fernet key.
func (m *Migrator) GenerateFernetKey() (string, error) {
	return services.GenerateKey()
}
