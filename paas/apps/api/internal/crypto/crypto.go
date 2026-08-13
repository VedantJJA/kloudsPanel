// Package crypto provides envelope encryption for secret values.
// The platform master key is an AES-256-GCM key loaded from environment.
// Each secret is encrypted with the master key; ciphertext, nonce, and
// key version are stored separately — never the plaintext.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

const KeyVersion = 1

// MasterKey holds the decoded 32-byte AES-256 key.
type MasterKey [32]byte

// ParseMasterKeyHex decodes a 64-hex-char string into a MasterKey.
// Returns an error if the input is missing or malformed.
func ParseMasterKeyHex(hexKey string) (MasterKey, error) {
	if hexKey == "" {
		return MasterKey{}, errors.New("MASTER_KEY_HEX is required (32 bytes, hex-encoded)")
	}
	b, err := hex.DecodeString(hexKey)
	if err != nil {
		return MasterKey{}, fmt.Errorf("invalid MASTER_KEY_HEX: %w", err)
	}
	if len(b) != 32 {
		return MasterKey{}, fmt.Errorf("MASTER_KEY_HEX must be 32 bytes (64 hex chars), got %d bytes", len(b))
	}
	var k MasterKey
	copy(k[:], b)
	return k, nil
}

// Envelope holds the stored fields for an encrypted secret.
type Envelope struct {
	Ciphertext []byte
	Nonce      []byte
	KeyVersion int
	AAD        []byte // additional authenticated data (e.g. secret ID)
}

// Encrypt encrypts plaintext with AES-256-GCM and returns an Envelope.
func Encrypt(key MasterKey, plaintext, aad []byte) (Envelope, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return Envelope{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return Envelope{}, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return Envelope{}, err
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, aad)
	return Envelope{
		Ciphertext: ciphertext,
		Nonce:      nonce,
		KeyVersion: KeyVersion,
		AAD:        aad,
	}, nil
}

// Decrypt decrypts an Envelope using AES-256-GCM and returns plaintext.
func Decrypt(key MasterKey, env Envelope) ([]byte, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plaintext, err := gcm.Open(nil, env.Nonce, env.Ciphertext, env.AAD)
	if err != nil {
		return nil, fmt.Errorf("decrypt: authentication failed")
	}
	return plaintext, nil
}

// GenerateKey generates a new random 32-byte key as a hex string.
func GenerateKey() (string, error) {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
