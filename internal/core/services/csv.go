package services

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"

	"github.com/flevanti/airflow-migrator/internal/core/models"
)

// CSV column headers - simple format: conn_id, encrypted_data
var csvHeaders = []string{
	"conn_id",
	"encrypted_data",
}

// ConnectionData holds all connection fields to be encrypted as a blob
type ConnectionData struct {
	ConnType         string `json:"conn_type"`
	Description      string `json:"description"`
	Host             string `json:"host"`
	Schema           string `json:"schema"`
	Login            string `json:"login"`
	Password         string `json:"password"`
	Port             int    `json:"port"`
	Extra            string `json:"extra"`
	IsEncrypted      bool   `json:"is_encrypted"`
	IsExtraEncrypted bool   `json:"is_extra_encrypted"`
	ExportedAt       string `json:"exported_at"`
}

// WriteEncryptedCSV writes connections to a CSV file with encrypted data.
func WriteEncryptedCSV(path string, records []*models.ExportRecord, fernet *Fernet) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	if err := writer.Write(csvHeaders); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Write records
	for _, r := range records {
		// Create data blob
		data := ConnectionData{
			ConnType:         r.ConnType,
			Description:      r.Description,
			Host:             r.Host,
			Schema:           r.Schema,
			Login:            r.Login,
			Password:         r.Password,
			Port:             r.Port,
			Extra:            r.Extra,
			IsEncrypted:      r.IsEncrypted,
			IsExtraEncrypted: r.IsExtraEncrypted,
			ExportedAt:       r.ExportedAt,
		}

		// Serialize to JSON
		jsonData, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("failed to serialize connection %s: %w", r.ConnID, err)
		}

		// Encrypt the JSON blob
		encrypted, err := fernet.EncryptString(string(jsonData))
		if err != nil {
			return fmt.Errorf("failed to encrypt connection %s: %w", r.ConnID, err)
		}

		row := []string{r.ConnID, encrypted}
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("failed to write row: %w", err)
		}
	}

	return writer.Error()
}

// ReadEncryptedCSV reads connections from an encrypted CSV file.
func ReadEncryptedCSV(path string, fernet *Fernet) ([]*models.ExportRecord, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)

	// Read all rows
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV: %w", err)
	}

	if len(rows) < 2 {
		return nil, nil // Empty file or header only
	}

	// Skip header, parse records
	var records []*models.ExportRecord
	for i, row := range rows[1:] {
		if len(row) < 2 {
			return nil, fmt.Errorf("invalid row %d: expected 2 columns", i+2)
		}

		connID := row[0]
		encryptedData := row[1]

		// Decrypt the blob
		decrypted, err := fernet.DecryptString(encryptedData)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt connection %s: %w", connID, err)
		}

		// Parse JSON
		var data ConnectionData
		if err := json.Unmarshal([]byte(decrypted), &data); err != nil {
			return nil, fmt.Errorf("failed to parse connection %s: %w", connID, err)
		}

		records = append(records, &models.ExportRecord{
			ConnID:           connID,
			ConnType:         data.ConnType,
			Description:      data.Description,
			Host:             data.Host,
			Schema:           data.Schema,
			Login:            data.Login,
			Password:         data.Password,
			Port:             data.Port,
			Extra:            data.Extra,
			IsEncrypted:      data.IsEncrypted,
			IsExtraEncrypted: data.IsExtraEncrypted,
			ExportedAt:       data.ExportedAt,
		})
	}

	return records, nil
}
