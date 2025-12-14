package models

import (
	"testing"
)

func TestConnection_Validate(t *testing.T) {
	tests := []struct {
		name    string
		conn    Connection
		wantErr bool
	}{
		{
			name:    "valid",
			conn:    Connection{ID: "test_conn", ConnType: "postgres"},
			wantErr: false,
		},
		{
			name:    "missing ID",
			conn:    Connection{ConnType: "postgres"},
			wantErr: true,
		},
		{
			name:    "missing type",
			conn:    Connection{ID: "test_conn"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.conn.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConnection_Clone(t *testing.T) {
	original := &Connection{
		ID:       "test",
		ConnType: "postgres",
		Host:     "localhost",
		Password: "secret",
	}

	clone := original.Clone()

	// Should be equal
	if clone.ID != original.ID || clone.Password != original.Password {
		t.Error("clone should have same values")
	}

	// Modifying clone shouldn't affect original
	clone.Password = "changed"
	if original.Password == "changed" {
		t.Error("modifying clone should not affect original")
	}
}

func TestConnection_ToExportRecord(t *testing.T) {
	conn := &Connection{
		ID:       "test_conn",
		ConnType: "postgres",
		Host:     "localhost",
		Port:     5432,
		Password: "secret",
	}

	record := conn.ToExportRecord()

	if record.ConnID != conn.ID {
		t.Errorf("ConnID: got %q, want %q", record.ConnID, conn.ID)
	}
	if record.ExportedAt == "" {
		t.Error("ExportedAt should be set")
	}
}

func TestExportRecord_ToConnection(t *testing.T) {
	record := &ExportRecord{
		ConnID:   "test_conn",
		ConnType: "postgres",
		Host:     "localhost",
		Port:     5432,
	}

	conn := record.ToConnection()

	if conn.ID != record.ConnID {
		t.Errorf("ID: got %q, want %q", conn.ID, record.ConnID)
	}
	if conn.IsEncrypted {
		t.Error("IsEncrypted should be false")
	}
}
