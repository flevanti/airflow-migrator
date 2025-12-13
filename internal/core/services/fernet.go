// Package services provides core business logic for the migrator.
package services

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"
)

var (
	ErrInvalidFernetKey   = errors.New("invalid fernet key: must be 32 bytes base64-encoded")
	ErrInvalidFernetToken = errors.New("invalid fernet token")
	ErrTokenExpired       = errors.New("fernet token expired")
	ErrInvalidHMAC        = errors.New("invalid HMAC signature")
)

const (
	fernetVersion      = 0x80
	fernetKeyLen       = 32
	fernetTimestampLen = 8
	fernetIVLen        = 16
)

// Fernet provides Python-compatible Fernet encryption/decryption.
type Fernet struct {
	signingKey    []byte // First 16 bytes
	encryptionKey []byte // Last 16 bytes
}

// NewFernet creates a Fernet instance from a base64-encoded key.
func NewFernet(base64Key string) (*Fernet, error) {
	key, err := base64.URLEncoding.DecodeString(base64Key)
	if err != nil {
		// Try standard base64
		key, err = base64.StdEncoding.DecodeString(base64Key)
		if err != nil {
			return nil, ErrInvalidFernetKey
		}
	}

	if len(key) != fernetKeyLen {
		return nil, ErrInvalidFernetKey
	}

	return &Fernet{
		signingKey:    key[:16],
		encryptionKey: key[16:],
	}, nil
}

// GenerateKey generates a new random Fernet key.
func GenerateKey() (string, error) {
	key := make([]byte, fernetKeyLen)
	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("failed to generate key: %w", err)
	}
	return base64.URLEncoding.EncodeToString(key), nil
}

// Encrypt encrypts plaintext and returns a Fernet token.
func (f *Fernet) Encrypt(plaintext []byte) (string, error) {
	// Generate IV
	iv := make([]byte, fernetIVLen)
	if _, err := rand.Read(iv); err != nil {
		return "", err
	}

	// Current timestamp
	timestamp := time.Now().Unix()

	// Pad plaintext (PKCS7)
	padded := pkcs7Pad(plaintext, aes.BlockSize)

	// Encrypt with AES-CBC
	block, err := aes.NewCipher(f.encryptionKey)
	if err != nil {
		return "", err
	}

	ciphertext := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, padded)

	// Build token: version || timestamp || iv || ciphertext
	token := make([]byte, 1+fernetTimestampLen+fernetIVLen+len(ciphertext))
	token[0] = fernetVersion
	putUint64BE(token[1:9], uint64(timestamp))
	copy(token[9:25], iv)
	copy(token[25:], ciphertext)

	// HMAC-SHA256
	h := hmac.New(sha256.New, f.signingKey)
	h.Write(token)
	signature := h.Sum(nil)

	// Final token: token || hmac
	final := append(token, signature...)

	return base64.URLEncoding.EncodeToString(final), nil
}

// Decrypt decrypts a Fernet token and returns plaintext.
func (f *Fernet) Decrypt(token string) ([]byte, error) {
	// Decode base64
	data, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		// Try standard base64
		data, err = base64.StdEncoding.DecodeString(token)
		if err != nil {
			return nil, ErrInvalidFernetToken
		}
	}

	// Minimum length: version(1) + timestamp(8) + iv(16) + block(16) + hmac(32)
	if len(data) < 73 {
		return nil, ErrInvalidFernetToken
	}

	// Verify version
	if data[0] != fernetVersion {
		return nil, ErrInvalidFernetToken
	}

	// Split: token data and HMAC
	tokenData := data[:len(data)-32]
	providedHMAC := data[len(data)-32:]

	// Verify HMAC
	h := hmac.New(sha256.New, f.signingKey)
	h.Write(tokenData)
	expectedHMAC := h.Sum(nil)

	if !hmac.Equal(providedHMAC, expectedHMAC) {
		return nil, ErrInvalidHMAC
	}

	// Extract IV and ciphertext
	iv := tokenData[9:25]
	ciphertext := tokenData[25:]

	// Decrypt with AES-CBC
	block, err := aes.NewCipher(f.encryptionKey)
	if err != nil {
		return nil, err
	}

	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, ErrInvalidFernetToken
	}

	plaintext := make([]byte, len(ciphertext))
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(plaintext, ciphertext)

	// Remove PKCS7 padding
	plaintext, err = pkcs7Unpad(plaintext)
	if err != nil {
		return nil, ErrInvalidFernetToken
	}

	return plaintext, nil
}

// DecryptString is a convenience method for string decryption.
func (f *Fernet) DecryptString(token string) (string, error) {
	plaintext, err := f.Decrypt(token)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// EncryptString is a convenience method for string encryption.
func (f *Fernet) EncryptString(plaintext string) (string, error) {
	return f.Encrypt([]byte(plaintext))
}

// ValidateKey checks if a string is a valid Fernet key.
func ValidateKey(key string) bool {
	_, err := NewFernet(key)
	return err == nil
}

// PKCS7 padding
func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - (len(data) % blockSize)
	padded := make([]byte, len(data)+padding)
	copy(padded, data)
	for i := len(data); i < len(padded); i++ {
		padded[i] = byte(padding)
	}
	return padded
}

func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("empty data")
	}
	padding := int(data[len(data)-1])
	if padding > len(data) || padding > aes.BlockSize {
		return nil, errors.New("invalid padding")
	}
	for i := len(data) - padding; i < len(data); i++ {
		if data[i] != byte(padding) {
			return nil, errors.New("invalid padding")
		}
	}
	return data[:len(data)-padding], nil
}

// Big-endian uint64
func putUint64BE(b []byte, v uint64) {
	b[0] = byte(v >> 56)
	b[1] = byte(v >> 48)
	b[2] = byte(v >> 40)
	b[3] = byte(v >> 32)
	b[4] = byte(v >> 24)
	b[5] = byte(v >> 16)
	b[6] = byte(v >> 8)
	b[7] = byte(v)
}
