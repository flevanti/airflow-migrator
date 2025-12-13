package services

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/flevanti/airflow-migrator/internal/core/models"
)

func TestCSV_WriteAndRead(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "csv-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	csvPath := filepath.Join(tmpDir, "test.csv")

	// Create test records
	records := []*models.ExportRecord{
		{
			ConnID:      "postgres_default",
			ConnType:    "postgres",
			Description: "Default Postgres",
			Host:        "localhost",
			Schema:      "airflow",
			Login:       "airflow",
			Password:    "encrypted_password_1",
			Port:        5432,
			Extra:       `{"sslmode": "disable"}`,
			ExportedAt:  "2024-01-15T10:30:00Z",
		},
		{
			ConnID:      "http_api",
			ConnType:    "http",
			Description: "External API",
			Host:        "api.example.com",
			Schema:      "",
			Login:       "api_user",
			Password:    "encrypted_password_2",
			Port:        443,
			Extra:       "",
			ExportedAt:  "2024-01-15T10:30:00Z",
		},
	}

	// Write
	if err := WriteCSV(csvPath, records); err != nil {
		t.Fatalf("WriteCSV failed: %v", err)
	}

	// Read back
	readRecords, err := ReadCSV(csvPath)
	if err != nil {
		t.Fatalf("ReadCSV failed: %v", err)
	}

	if len(readRecords) != len(records) {
		t.Fatalf("expected %d records, got %d", len(records), len(readRecords))
	}

	// Verify first record
	if readRecords[0].ConnID != records[0].ConnID {
		t.Errorf("ConnID: got %q, want %q", readRecords[0].ConnID, records[0].ConnID)
	}
	if readRecords[0].Password != records[0].Password {
		t.Errorf("Password: got %q, want %q", readRecords[0].Password, records[0].Password)
	}
	if readRecords[0].Port != records[0].Port {
		t.Errorf("Port: got %d, want %d", readRecords[0].Port, records[0].Port)
	}
	if readRecords[0].Extra != records[0].Extra {
		t.Errorf("Extra: got %q, want %q", readRecords[0].Extra, records[0].Extra)
	}
}

func TestCSV_EmptyFile(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "csv-test-*")
	defer os.RemoveAll(tmpDir)

	csvPath := filepath.Join(tmpDir, "empty.csv")

	// Write empty
	if err := WriteCSV(csvPath, nil); err != nil {
		t.Fatalf("WriteCSV failed: %v", err)
	}

	// Read back
	records, err := ReadCSV(csvPath)
	if err != nil {
		t.Fatalf("ReadCSV failed: %v", err)
	}

	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}
}

func TestCSV_SpecialCharacters(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "csv-test-*")
	defer os.RemoveAll(tmpDir)

	csvPath := filepath.Join(tmpDir, "special.csv")

	records := []*models.ExportRecord{
		{
			ConnID:      "special_conn",
			ConnType:    "postgres",
			Description: "Has, commas and \"quotes\"",
			Host:        "localhost",
			Password:    "pass,with,commas",
			Extra:       `{"key": "value, with comma"}`,
		},
	}

	if err := WriteCSV(csvPath, records); err != nil {
		t.Fatalf("WriteCSV failed: %v", err)
	}

	readRecords, err := ReadCSV(csvPath)
	if err != nil {
		t.Fatalf("ReadCSV failed: %v", err)
	}

	if readRecords[0].Description != records[0].Description {
		t.Errorf("Description: got %q, want %q", readRecords[0].Description, records[0].Description)
	}
	if readRecords[0].Password != records[0].Password {
		t.Errorf("Password: got %q, want %q", readRecords[0].Password, records[0].Password)
	}
}

func TestCSV_FileNotFound(t *testing.T) {
	_, err := ReadCSV("/nonexistent/path/file.csv")
	if err == nil {
		t.Error("should fail for nonexistent file")
	}
}
