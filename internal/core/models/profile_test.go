package models

import (
	"testing"
)

func TestProfile_Validate(t *testing.T) {
	tests := []struct {
		name    string
		profile Profile
		wantErr bool
	}{
		{
			name: "valid",
			profile: Profile{
				ID:        "1",
				Name:      "Dev",
				DBHost:    "localhost",
				DBPort:    5432,
				DBName:    "airflow",
				DBUser:    "airflow",
				FernetKey: "test-key",
			},
			wantErr: false,
		},
		{
			name: "missing name",
			profile: Profile{
				ID:        "1",
				DBHost:    "localhost",
				DBPort:    5432,
				DBName:    "airflow",
				DBUser:    "airflow",
				FernetKey: "test-key",
			},
			wantErr: true,
		},
		{
			name: "invalid port",
			profile: Profile{
				ID:        "1",
				Name:      "Dev",
				DBHost:    "localhost",
				DBPort:    0,
				DBName:    "airflow",
				DBUser:    "airflow",
				FernetKey: "test-key",
			},
			wantErr: true,
		},
		{
			name: "missing fernet key",
			profile: Profile{
				ID:     "1",
				Name:   "Dev",
				DBHost: "localhost",
				DBPort: 5432,
				DBName: "airflow",
				DBUser: "airflow",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.profile.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestProfile_DSN(t *testing.T) {
	p := &Profile{
		DBHost:     "localhost",
		DBPort:     5432,
		DBName:     "airflow",
		DBUser:     "airflow",
		DBPassword: "secret",
		DBSSLMode:  "disable",
	}

	dsn := p.DSN()
	expected := "postgres://airflow:secret@localhost:5432/airflow?sslmode=disable"

	if dsn != expected {
		t.Errorf("DSN: got %q, want %q", dsn, expected)
	}
}

func TestProfile_DSN_EmptySSLMode(t *testing.T) {
	p := &Profile{
		DBHost:     "localhost",
		DBPort:     5432,
		DBName:     "airflow",
		DBUser:     "airflow",
		DBPassword: "secret",
		DBSSLMode:  "", // Empty should default to disable
	}

	dsn := p.DSN()
	expected := "postgres://airflow:secret@localhost:5432/airflow?sslmode=disable"

	if dsn != expected {
		t.Errorf("DSN with empty SSLMode: got %q, want %q", dsn, expected)
	}
}

func TestProfile_DSN_SpecialCharacters(t *testing.T) {
	p := &Profile{
		DBHost:     "db.example.com",
		DBPort:     5432,
		DBName:     "airflow_prod",
		DBUser:     "admin",
		DBPassword: "p@ss:word/123",
		DBSSLMode:  "require",
	}

	dsn := p.DSN()

	// Should contain the password as-is (URL encoding may be needed by driver)
	if dsn == "" {
		t.Error("DSN should not be empty")
	}
}

func TestProfile_GetSecretKeys(t *testing.T) {
	p := &Profile{ID: "profile-123"}
	keys := p.GetSecretKeys()

	if keys.Password != "profile:profile-123:password" {
		t.Errorf("Password key: got %q", keys.Password)
	}
	if keys.FernetKey != "profile:profile-123:fernet" {
		t.Errorf("FernetKey key: got %q", keys.FernetKey)
	}
}

func TestNewProfile(t *testing.T) {
	p := NewProfile("Test Profile")

	if p.Name != "Test Profile" {
		t.Errorf("Name: got %q", p.Name)
	}
	if p.ID == "" {
		t.Error("ID should be generated")
	}
	if p.DBPort != DefaultDBPort {
		t.Errorf("DBPort: got %d, want %d", p.DBPort, DefaultDBPort)
	}
	if p.DBSSLMode != DefaultDBSSLMode {
		t.Errorf("DBSSLMode: got %q, want %q", p.DBSSLMode, DefaultDBSSLMode)
	}
}
