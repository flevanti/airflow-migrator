package services

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/flevanti/airflow-migrator/internal/core/models"
)

func TestCSV_WriteAndReadEncrypted(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "csv-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	csvPath := filepath.Join(tmpDir, "test.csv")

	// Create a Fernet key for testing
	key, err := GenerateKey()
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	fernet, err := NewFernet(key)
	if err != nil {
		t.Fatalf("failed to create fernet: %v", err)
	}

	// Create test records
	records := []*models.ExportRecord{
		{
			ConnID:      "postgres_default",
			ConnType:    "postgres",
			Description: "Default Postgres",
			Host:        "localhost",
			Schema:      "airflow",
			Login:       "airflow",
			Password:    "secret_password",
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
			Password:    "api_secret",
			Port:        443,
			Extra:       "",
			ExportedAt:  "2024-01-15T10:30:00Z",
		},
	}

	// Write encrypted
	if err := WriteEncryptedCSV(csvPath, records, fernet); err != nil {
		t.Fatalf("WriteEncryptedCSV failed: %v", err)
	}

	// Read and decrypt
	readRecords, err := ReadEncryptedCSV(csvPath, fernet)
	if err != nil {
		t.Fatalf("ReadEncryptedCSV failed: %v", err)
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

func TestCSV_WrongKey(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "csv-test-*")
	defer os.RemoveAll(tmpDir)

	csvPath := filepath.Join(tmpDir, "test.csv")

	// Create two different keys
	key1, _ := GenerateKey()
	key2, _ := GenerateKey()
	fernet1, _ := NewFernet(key1)
	fernet2, _ := NewFernet(key2)

	records := []*models.ExportRecord{
		{ConnID: "test", ConnType: "postgres", Password: "secret"},
	}

	// Write with key1
	if err := WriteEncryptedCSV(csvPath, records, fernet1); err != nil {
		t.Fatalf("WriteEncryptedCSV failed: %v", err)
	}

	// Try to read with key2 - should fail
	_, err := ReadEncryptedCSV(csvPath, fernet2)
	if err == nil {
		t.Error("should fail with wrong key")
	}
}

func TestCSV_EmptyFile(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "csv-test-*")
	defer os.RemoveAll(tmpDir)

	csvPath := filepath.Join(tmpDir, "empty.csv")
	key, _ := GenerateKey()
	fernet, _ := NewFernet(key)

	// Write empty
	if err := WriteEncryptedCSV(csvPath, nil, fernet); err != nil {
		t.Fatalf("WriteEncryptedCSV failed: %v", err)
	}

	// Read back
	records, err := ReadEncryptedCSV(csvPath, fernet)
	if err != nil {
		t.Fatalf("ReadEncryptedCSV failed: %v", err)
	}

	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}
}

func TestCSV_SpecialCharacters(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "csv-test-*")
	defer os.RemoveAll(tmpDir)

	csvPath := filepath.Join(tmpDir, "special.csv")
	key, _ := GenerateKey()
	fernet, _ := NewFernet(key)

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

	if err := WriteEncryptedCSV(csvPath, records, fernet); err != nil {
		t.Fatalf("WriteEncryptedCSV failed: %v", err)
	}

	readRecords, err := ReadEncryptedCSV(csvPath, fernet)
	if err != nil {
		t.Fatalf("ReadEncryptedCSV failed: %v", err)
	}

	if readRecords[0].Description != records[0].Description {
		t.Errorf("Description: got %q, want %q", readRecords[0].Description, records[0].Description)
	}
	if readRecords[0].Password != records[0].Password {
		t.Errorf("Password: got %q, want %q", readRecords[0].Password, records[0].Password)
	}
}

func TestCSV_FileNotFound(t *testing.T) {
	key, _ := GenerateKey()
	fernet, _ := NewFernet(key)

	_, err := ReadEncryptedCSV("/nonexistent/path/file.csv", fernet)
	if err == nil {
		t.Error("should fail for nonexistent file")
	}
}

func TestCSV_EncryptionFlags(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "csv-test-*")
	defer os.RemoveAll(tmpDir)

	csvPath := filepath.Join(tmpDir, "flags.csv")
	key, _ := GenerateKey()
	fernet, _ := NewFernet(key)

	records := []*models.ExportRecord{
		{
			ConnID:           "encrypted_conn",
			ConnType:         "postgres",
			Password:         "secret",
			Extra:            `{"key": "value"}`,
			IsEncrypted:      true,
			IsExtraEncrypted: true,
		},
		{
			ConnID:           "unencrypted_conn",
			ConnType:         "http",
			Password:         "plain",
			Extra:            "",
			IsEncrypted:      false,
			IsExtraEncrypted: false,
		},
	}

	if err := WriteEncryptedCSV(csvPath, records, fernet); err != nil {
		t.Fatalf("WriteEncryptedCSV failed: %v", err)
	}

	readRecords, err := ReadEncryptedCSV(csvPath, fernet)
	if err != nil {
		t.Fatalf("ReadEncryptedCSV failed: %v", err)
	}

	// Check encryption flags preserved
	if readRecords[0].IsEncrypted != true {
		t.Error("IsEncrypted should be true for first record")
	}
	if readRecords[0].IsExtraEncrypted != true {
		t.Error("IsExtraEncrypted should be true for first record")
	}
	if readRecords[1].IsEncrypted != false {
		t.Error("IsEncrypted should be false for second record")
	}
	if readRecords[1].IsExtraEncrypted != false {
		t.Error("IsExtraEncrypted should be false for second record")
	}
}

func TestCSV_LargeDataSet(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "csv-test-*")
	defer os.RemoveAll(tmpDir)

	csvPath := filepath.Join(tmpDir, "large.csv")
	key, _ := GenerateKey()
	fernet, _ := NewFernet(key)

	// Create 100 records
	records := make([]*models.ExportRecord, 100)
	for i := 0; i < 100; i++ {
		records[i] = &models.ExportRecord{
			ConnID:   fmt.Sprintf("conn_%03d", i),
			ConnType: "postgres",
			Host:     "localhost",
			Password: fmt.Sprintf("password_%d", i),
			Port:     5432 + i,
		}
	}

	if err := WriteEncryptedCSV(csvPath, records, fernet); err != nil {
		t.Fatalf("WriteEncryptedCSV failed: %v", err)
	}

	readRecords, err := ReadEncryptedCSV(csvPath, fernet)
	if err != nil {
		t.Fatalf("ReadEncryptedCSV failed: %v", err)
	}

	if len(readRecords) != 100 {
		t.Errorf("expected 100 records, got %d", len(readRecords))
	}

	// Verify first and last
	if readRecords[0].ConnID != "conn_000" {
		t.Errorf("first ConnID: got %q", readRecords[0].ConnID)
	}
	if readRecords[99].ConnID != "conn_099" {
		t.Errorf("last ConnID: got %q", readRecords[99].ConnID)
	}
}
