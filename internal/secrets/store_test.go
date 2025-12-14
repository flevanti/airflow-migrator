package secrets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStore_NewAndOpen(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "secrets-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	password := "test-password-123"

	// Create new store
	store, err := New(tmpDir, password)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// Set a value
	if err := store.Set("key1", "value1"); err != nil {
		t.Fatalf("Set() failed: %v", err)
	}

	// Close and reopen
	store = nil

	store2, err := New(tmpDir, password)
	if err != nil {
		t.Fatalf("reopening store failed: %v", err)
	}

	// Value should persist
	val, err := store2.Get("key1")
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}
	if val != "value1" {
		t.Errorf("Get(): got %q, want %q", val, "value1")
	}
}

func TestStore_WrongPassword(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "secrets-test-*")
	defer os.RemoveAll(tmpDir)

	// Create store with password
	store, err := New(tmpDir, "correct-password")
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	store.Set("key", "value")
	store = nil

	// Try to open with wrong password
	_, err = New(tmpDir, "wrong-password")
	if err == nil {
		t.Error("should fail with wrong password")
	}
}

func TestStore_CRUD(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "secrets-test-*")
	defer os.RemoveAll(tmpDir)

	store, _ := New(tmpDir, "password")

	// Set
	if err := store.Set("key1", "value1"); err != nil {
		t.Fatalf("Set() failed: %v", err)
	}

	// Get
	val, err := store.Get("key1")
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}
	if val != "value1" {
		t.Errorf("Get(): got %q, want %q", val, "value1")
	}

	// Update
	if err := store.Set("key1", "updated"); err != nil {
		t.Fatalf("Set() update failed: %v", err)
	}
	val, _ = store.Get("key1")
	if val != "updated" {
		t.Errorf("Get() after update: got %q, want %q", val, "updated")
	}

	// Delete
	if err := store.Delete("key1"); err != nil {
		t.Fatalf("Delete() failed: %v", err)
	}

	// Get deleted - should fail
	_, err = store.Get("key1")
	if err == nil {
		t.Error("Get() deleted key should fail")
	}
}

func TestStore_List(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "secrets-test-*")
	defer os.RemoveAll(tmpDir)

	store, _ := New(tmpDir, "password")

	store.Set("profile:1:meta", "data1")
	store.Set("profile:1:password", "pass1")
	store.Set("profile:2:meta", "data2")
	store.Set("other:key", "value")

	keys := store.List()

	if len(keys) != 4 {
		t.Errorf("List(): got %d keys, want 4", len(keys))
	}

	// Check all keys present
	keyMap := make(map[string]bool)
	for _, k := range keys {
		keyMap[k] = true
	}

	expected := []string{"profile:1:meta", "profile:1:password", "profile:2:meta", "other:key"}
	for _, k := range expected {
		if !keyMap[k] {
			t.Errorf("List() missing key: %s", k)
		}
	}
}

func TestStore_GetNonexistent(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "secrets-test-*")
	defer os.RemoveAll(tmpDir)

	store, _ := New(tmpDir, "password")

	_, err := store.Get("nonexistent")
	if err == nil {
		t.Error("Get() nonexistent key should fail")
	}
}

func TestStore_SpecialCharacters(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "secrets-test-*")
	defer os.RemoveAll(tmpDir)

	store, _ := New(tmpDir, "password")

	tests := []struct {
		key   string
		value string
	}{
		{"simple", "simple value"},
		{"with:colons", "value:with:colons"},
		{"unicode-key-🔑", "unicode-value-🔐"},
		{"json-value", `{"host": "localhost", "port": 5432}`},
		{"multiline", "line1\nline2\nline3"},
		{"special", "!@#$%^&*()_+-=[]{}|;':\",./<>?"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			if err := store.Set(tt.key, tt.value); err != nil {
				t.Fatalf("Set() failed: %v", err)
			}

			val, err := store.Get(tt.key)
			if err != nil {
				t.Fatalf("Get() failed: %v", err)
			}

			if val != tt.value {
				t.Errorf("got %q, want %q", val, tt.value)
			}
		})
	}
}

func TestStore_Exists(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "secrets-test-*")
	defer os.RemoveAll(tmpDir)

	// Should not exist initially
	if Exists(tmpDir) {
		t.Error("Exists() should be false for new dir")
	}

	// Create store and set a value (this writes the file)
	store, _ := New(tmpDir, "password")
	store.Set("key", "value")

	// Should exist now
	if !Exists(tmpDir) {
		t.Error("Exists() should be true after setting a value")
	}
}

func TestStore_EmptyPassword(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "secrets-test-*")
	defer os.RemoveAll(tmpDir)

	// Empty password should still work (not recommended but valid)
	store, err := New(tmpDir, "")
	if err != nil {
		t.Fatalf("New() with empty password failed: %v", err)
	}

	store.Set("key", "value")
	val, _ := store.Get("key")
	if val != "value" {
		t.Errorf("got %q, want %q", val, "value")
	}
}

func TestStore_Persistence(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "secrets-test-*")
	defer os.RemoveAll(tmpDir)

	password := "persistence-test"

	// Create and populate
	store1, _ := New(tmpDir, password)
	store1.Set("persist1", "value1")
	store1.Set("persist2", "value2")
	store1 = nil

	// Reopen
	store2, _ := New(tmpDir, password)

	// Verify all data
	v1, _ := store2.Get("persist1")
	v2, _ := store2.Get("persist2")

	if v1 != "value1" || v2 != "value2" {
		t.Error("data not persisted correctly")
	}

	// Modify and close
	store2.Set("persist3", "value3")
	store2.Delete("persist1")
	store2 = nil

	// Reopen again
	store3, _ := New(tmpDir, password)

	_, err := store3.Get("persist1")
	if err == nil {
		t.Error("deleted key should not exist")
	}

	v3, _ := store3.Get("persist3")
	if v3 != "value3" {
		t.Error("new key not persisted")
	}
}

func TestStore_CredentialsFilePath(t *testing.T) {
	tmpDir, _ := os.MkdirTemp("", "secrets-test-*")
	defer os.RemoveAll(tmpDir)

	store, _ := New(tmpDir, "password")
	store.Set("key", "value") // This triggers file write

	// Check file exists
	credFile := filepath.Join(tmpDir, "credentials.enc")
	if _, err := os.Stat(credFile); os.IsNotExist(err) {
		t.Error("credentials.enc should be created after Set()")
	}
}
