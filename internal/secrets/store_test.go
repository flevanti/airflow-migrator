package secrets

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStore_BasicOperations(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "secrets-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	masterPassword := "test-master-password-123"

	// Create new store
	store, err := New(tmpDir, masterPassword)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Test Set and Get
	t.Run("Set and Get", func(t *testing.T) {
		err := store.Set("fernet-key-1", "my-secret-fernet-key")
		if err != nil {
			t.Fatalf("failed to set value: %v", err)
		}

		value, err := store.Get("fernet-key-1")
		if err != nil {
			t.Fatalf("failed to get value: %v", err)
		}

		if value != "my-secret-fernet-key" {
			t.Errorf("expected 'my-secret-fernet-key', got '%s'", value)
		}
	})

	// Test Has
	t.Run("Has", func(t *testing.T) {
		if !store.Has("fernet-key-1") {
			t.Error("expected Has to return true for existing key")
		}

		if store.Has("nonexistent") {
			t.Error("expected Has to return false for nonexistent key")
		}
	})

	// Test List
	t.Run("List", func(t *testing.T) {
		store.Set("db-password", "secret123")
		store.Set("api-key", "apikey456")

		keys := store.List()
		if len(keys) != 3 {
			t.Errorf("expected 3 keys, got %d", len(keys))
		}
	})

	// Test Delete
	t.Run("Delete", func(t *testing.T) {
		err := store.Delete("api-key")
		if err != nil {
			t.Fatalf("failed to delete: %v", err)
		}

		if store.Has("api-key") {
			t.Error("key should not exist after delete")
		}
	})

	// Test Get nonexistent key
	t.Run("Get nonexistent", func(t *testing.T) {
		_, err := store.Get("nonexistent")
		if err != ErrKeyNotFound {
			t.Errorf("expected ErrKeyNotFound, got %v", err)
		}
	})
}

func TestStore_Persistence(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "secrets-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	masterPassword := "persistence-test-password"

	// Create store and add data
	store1, err := New(tmpDir, masterPassword)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	store1.Set("profile:dev:fernet", "dev-fernet-key-12345")
	store1.Set("profile:dev:password", "dev-db-password")

	// Create new store instance with same password - should load existing data
	store2, err := New(tmpDir, masterPassword)
	if err != nil {
		t.Fatalf("failed to create second store: %v", err)
	}

	value, err := store2.Get("profile:dev:fernet")
	if err != nil {
		t.Fatalf("failed to get value from reloaded store: %v", err)
	}

	if value != "dev-fernet-key-12345" {
		t.Errorf("expected 'dev-fernet-key-12345', got '%s'", value)
	}
}

func TestStore_WrongPassword(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "secrets-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create store with one password
	store1, err := New(tmpDir, "correct-password")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	store1.Set("secret", "value")

	// Try to open with wrong password
	_, err = New(tmpDir, "wrong-password")
	if err != ErrInvalidPassword {
		t.Errorf("expected ErrInvalidPassword, got %v", err)
	}
}

func TestStore_Clear(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "secrets-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := New(tmpDir, "test-password")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	store.Set("key1", "value1")
	store.Set("key2", "value2")

	if len(store.List()) != 2 {
		t.Error("expected 2 keys before clear")
	}

	store.Clear()

	if len(store.List()) != 0 {
		t.Error("expected 0 keys after clear")
	}
}

func TestExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "secrets-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Should not exist initially
	if Exists(tmpDir) {
		t.Error("expected Exists to return false before store creation")
	}

	// Create store and add something
	store, _ := New(tmpDir, "test")
	store.Set("key", "value")

	// Should exist now
	if !Exists(tmpDir) {
		t.Error("expected Exists to return true after store creation")
	}
}

func TestStore_SpecialCharacters(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "secrets-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := New(tmpDir, "test-password")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Test with special characters, unicode, and base64-like strings
	testCases := map[string]string{
		"unicode-key":     "こんにちは世界",
		"special-chars":   "!@#$%^&*()_+-=[]{}|;':\",./<>?",
		"base64-fernet":   "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eXo=",
		"newlines":        "line1\nline2\nline3",
		"empty-value-key": "",
	}

	for key, value := range testCases {
		err := store.Set(key, value)
		if err != nil {
			t.Errorf("failed to set %s: %v", key, err)
			continue
		}

		got, err := store.Get(key)
		if err != nil {
			t.Errorf("failed to get %s: %v", key, err)
			continue
		}

		if got != value {
			t.Errorf("key %s: expected %q, got %q", key, value, got)
		}
	}
}

func TestStore_FilesCreated(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "secrets-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := New(tmpDir, "test-password")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Add data to trigger file creation
	store.Set("key", "value")

	// Check salt file exists
	saltPath := filepath.Join(tmpDir, "salt")
	if _, err := os.Stat(saltPath); os.IsNotExist(err) {
		t.Error("salt file should exist")
	}

	// Check credentials file exists
	credPath := filepath.Join(tmpDir, "credentials.enc")
	if _, err := os.Stat(credPath); os.IsNotExist(err) {
		t.Error("credentials.enc file should exist")
	}

	// Verify file permissions (should be 0600)
	info, _ := os.Stat(credPath)
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected permissions 0600, got %o", info.Mode().Perm())
	}
}
