// Package secrets provides encrypted storage for sensitive credentials.
// It uses AES-256-GCM encryption with a key derived from a master password using Argon2.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/crypto/argon2"
)

const (
	// Argon2 parameters
	argonTime    = 1
	argonMemory  = 64 * 1024 // 64MB
	argonThreads = 4
	argonKeyLen  = 32 // AES-256

	// File names
	credentialsFile = "credentials.enc"
	saltFile        = "salt"
	saltLength      = 16
)

var (
	ErrInvalidPassword = errors.New("invalid master password")
	ErrKeyNotFound     = errors.New("key not found")
	ErrNotInitialized  = errors.New("store not initialized")
)

// Store provides encrypted storage for sensitive data.
// All operations are thread-safe.
type Store struct {
	mu       sync.RWMutex
	key      []byte            // Derived encryption key
	filePath string            // Path to encrypted file
	saltPath string            // Path to salt file
	data     map[string]string // Decrypted data in memory
}

// New creates a new secret store. If the store already exists, it decrypts it
// using the provided master password. If it doesn't exist, it creates a new one.
func New(configDir string, masterPassword string) (*Store, error) {
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	s := &Store{
		filePath: filepath.Join(configDir, credentialsFile),
		saltPath: filepath.Join(configDir, saltFile),
		data:     make(map[string]string),
	}

	// Get or create salt
	salt, err := s.getOrCreateSalt()
	if err != nil {
		return nil, fmt.Errorf("failed to get salt: %w", err)
	}

	// Derive key from password
	s.key = argon2.IDKey(
		[]byte(masterPassword),
		salt,
		argonTime,
		argonMemory,
		argonThreads,
		argonKeyLen,
	)

	// Load existing data if file exists
	if _, err := os.Stat(s.filePath); err == nil {
		if err := s.load(); err != nil {
			return nil, err
		}
	}

	return s, nil
}

// getOrCreateSalt retrieves existing salt or creates a new one
func (s *Store) getOrCreateSalt() ([]byte, error) {
	// Try to read existing salt
	if salt, err := os.ReadFile(s.saltPath); err == nil && len(salt) == saltLength {
		return salt, nil
	}

	// Generate new salt
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}

	// Save salt
	if err := os.WriteFile(s.saltPath, salt, 0600); err != nil {
		return nil, fmt.Errorf("failed to save salt: %w", err)
	}

	return salt, nil
}

// load decrypts and loads data from the encrypted file
func (s *Store) load() error {
	ciphertext, err := os.ReadFile(s.filePath)
	if err != nil {
		return fmt.Errorf("failed to read credentials file: %w", err)
	}

	plaintext, err := s.decrypt(ciphertext)
	if err != nil {
		return ErrInvalidPassword
	}

	if err := json.Unmarshal(plaintext, &s.data); err != nil {
		return ErrInvalidPassword
	}

	return nil
}

// save encrypts and saves data to the encrypted file
func (s *Store) save() error {
	plaintext, err := json.Marshal(s.data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	ciphertext, err := s.encrypt(plaintext)
	if err != nil {
		return fmt.Errorf("failed to encrypt data: %w", err)
	}

	if err := os.WriteFile(s.filePath, ciphertext, 0600); err != nil {
		return fmt.Errorf("failed to write credentials file: %w", err)
	}

	return nil
}

// encrypt encrypts data using AES-256-GCM
func (s *Store) encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	// Prepend nonce to ciphertext
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// decrypt decrypts data using AES-256-GCM
func (s *Store) decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < gcm.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}

	nonce := ciphertext[:gcm.NonceSize()]
	ciphertext = ciphertext[gcm.NonceSize():]

	return gcm.Open(nil, nonce, ciphertext, nil)
}

// Get retrieves a value by key
func (s *Store) Get(key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	value, ok := s.data[key]
	if !ok {
		return "", ErrKeyNotFound
	}

	return value, nil
}

// Set stores a key-value pair and persists to disk
func (s *Store) Set(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data[key] = value
	return s.save()
}

// Delete removes a key and persists to disk
func (s *Store) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data[key]; !ok {
		return ErrKeyNotFound
	}

	delete(s.data, key)
	return s.save()
}

// List returns all keys in the store
func (s *Store) List() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}

	return keys
}

// Has checks if a key exists
func (s *Store) Has(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, ok := s.data[key]
	return ok
}

// Clear removes all data and persists to disk
func (s *Store) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.data = make(map[string]string)
	return s.save()
}

// Exists checks if a credentials file already exists
func Exists(configDir string) bool {
	filePath := filepath.Join(configDir, credentialsFile)
	_, err := os.Stat(filePath)
	return err == nil
}
