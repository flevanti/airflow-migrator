package core

import (
	"testing"
)

func TestMigrator_GenerateFernetKey(t *testing.T) {
	m := New()

	key, err := m.GenerateFernetKey()
	if err != nil {
		t.Fatalf("GenerateFernetKey() failed: %v", err)
	}

	if key == "" {
		t.Error("generated key should not be empty")
	}

	// Should be valid
	if !m.ValidateFernetKey(key) {
		t.Error("generated key should be valid")
	}

	// Two keys should be different
	key2, _ := m.GenerateFernetKey()
	if key == key2 {
		t.Error("two generated keys should be different")
	}
}

func TestMigrator_ValidateFernetKey(t *testing.T) {
	m := New()

	tests := []struct {
		name  string
		key   string
		valid bool
	}{
		{"valid key", "cw_0x689RpI-jtRR7oE8h_eQsKImvJapLeSbXpwF4e4=", true},
		{"too short", "dGVzdA==", false},
		{"not base64", "not-valid-base64!!!", false},
		{"empty", "", false},
		{"almost valid", "cw_0x689RpI-jtRR7oE8h_eQsKImvJapLeSbXpwF4e4", false}, // missing =
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := m.ValidateFernetKey(tt.key); got != tt.valid {
				t.Errorf("ValidateFernetKey() = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestMigrator_New(t *testing.T) {
	m := New()
	if m == nil {
		t.Error("New() should return non-nil migrator")
	}
}
