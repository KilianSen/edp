// Package secret provides authenticated encryption (AES-256-GCM) for sensitive
// fields stored at rest (here: each edp instance's API token). The key lives in
// a 0600 file in the data dir, separate from the SQLite DB. Ciphertext is tagged
// with a version prefix so plaintext written by older versions still reads back.
package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	prefix  = "enc:v1:"
	keyName = "secret.key"
)

type Box struct {
	gcm cipher.AEAD
}

// Load reads the key file in dir, creating a fresh 32-byte key on first run.
func Load(dir string) (*Box, error) {
	path := filepath.Join(dir, keyName)
	key, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		key = make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, key, 0o600); err != nil {
			return nil, fmt.Errorf("write key: %w", err)
		}
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("secret key must be 32 bytes, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{gcm: gcm}, nil
}

// Encrypt returns a versioned ciphertext string. Empty input stays empty, and
// already-encrypted input is returned unchanged (idempotent).
func (b *Box) Encrypt(plain string) (string, error) {
	if plain == "" || strings.HasPrefix(plain, prefix) {
		return plain, nil
	}
	nonce := make([]byte, b.gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ct := b.gcm.Seal(nonce, nonce, []byte(plain), nil)
	return prefix + base64.StdEncoding.EncodeToString(ct), nil
}

// Decrypt reverses Encrypt. Values without the version prefix are assumed to be
// legacy plaintext and returned as-is, so existing rows keep working.
func (b *Box) Decrypt(stored string) (string, error) {
	rest, ok := strings.CutPrefix(stored, prefix)
	if !ok {
		return stored, nil
	}
	raw, err := base64.StdEncoding.DecodeString(rest)
	if err != nil {
		return "", err
	}
	ns := b.gcm.NonceSize()
	if len(raw) < ns {
		return "", fmt.Errorf("ciphertext too short")
	}
	plain, err := b.gcm.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}
	return string(plain), nil
}
