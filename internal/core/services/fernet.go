// Package services provides core business logic for the migrator.
package services

import (
	"errors"

	"github.com/fernet/fernet-go"
)

var (
	ErrInvalidFernetKey   = errors.New("invalid fernet key: must be 32 bytes base64-encoded")
	ErrInvalidFernetToken = errors.New("invalid fernet token")
)

// Fernet provides Python-compatible Fernet encryption/decryption.
type Fernet struct {
	key *fernet.Key
}

// NewFernet creates a Fernet instance from a base64-encoded key.
func NewFernet(base64Key string) (*Fernet, error) {
	key, err := fernet.DecodeKey(base64Key)
	if err != nil {
		return nil, ErrInvalidFernetKey
	}

	return &Fernet{key: key}, nil
}

// GenerateKey generates a new random Fernet key.
func GenerateKey() (string, error) {
	key := fernet.Key{}
	if err := key.Generate(); err != nil {
		return "", err
	}
	return key.Encode(), nil
}

// Encrypt encrypts plaintext and returns a Fernet token.
func (f *Fernet) Encrypt(plaintext []byte) (string, error) {
	token, err := fernet.EncryptAndSign(plaintext, f.key)
	if err != nil {
		return "", err
	}
	return string(token), nil
}

// Decrypt decrypts a Fernet token and returns plaintext.
func (f *Fernet) Decrypt(token string) ([]byte, error) {
	msg := fernet.VerifyAndDecrypt([]byte(token), 0, []*fernet.Key{f.key})
	if msg == nil {
		return nil, ErrInvalidFernetToken
	}
	return msg, nil
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
