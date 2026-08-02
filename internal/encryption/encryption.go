// Package encryption provides per-entry AES-256-GCM encryption and decryption.
//
// Design note: this package only ever deals with keys that are handed to it
// by the caller (the service layer). It never stores or remembers a key
// itself - that is the whole point of the "returns encryption key" /
// "requires encryption key" contract in the kv-store spec. If the caller
// loses the key, the data is unrecoverable; if the underlying storage is
// dumped, the ciphertext is useless without a key that was never persisted.
//
// This package deliberately knows nothing about how or where its output is
// stored - it just takes bytes in and returns bytes out. The storage.Entry
// type (ciphertext + nonce bundled together) is owned by the storage
// package; the service package is what connects the two.
package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
)

// KeySize is the AES-256 key size in bytes.
const KeySize = 32 // 256 bits

var ErrDecryptionFailed = errors.New("encryption: decryption failed (wrong key or corrupted data)")

// GenerateKey creates a new random 256-bit key suitable for AES-256-GCM.
func GenerateKey() ([]byte, error) {
	key := make([]byte, KeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("encryption: failed to generate key: %w", err)
	}
	return key, nil
}

// Encrypt encrypts plaintext with the given key using AES-256-GCM. It
// returns the ciphertext and the randomly generated nonce that was used,
// as two separate values - the caller decides how to store/bundle them.
func Encrypt(plaintext, key []byte) (ciphertext, nonce []byte, err error) {
	if len(key) != KeySize {
		return nil, nil, fmt.Errorf("encryption: key must be %d bytes, got %d", KeySize, len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, fmt.Errorf("encryption: failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("encryption: failed to create GCM: %w", err)
	}

	nonce = make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, fmt.Errorf("encryption: failed to generate nonce: %w", err)
	}

	// dst = nil means "start with an empty slice" - ciphertext contains
	// only the encrypted bytes + auth tag, nothing prepended.
	ciphertext = gcm.Seal(nil, nonce, plaintext, nil)

	return ciphertext, nonce, nil
}

// Decrypt reverses Encrypt: given the ciphertext, the nonce that was used
// to produce it, and the original key, it returns the original plaintext.
// Returns ErrDecryptionFailed if the key is wrong or the data has been
// tampered with/corrupted - GCM authenticates the ciphertext, so this is a
// reliable check, not a guess.
func Decrypt(ciphertext, nonce, key []byte) ([]byte, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("encryption: key must be %d bytes, got %d", KeySize, len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("encryption: failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("encryption: failed to create GCM: %w", err)
	}

	if len(nonce) != gcm.NonceSize() {
		return nil, ErrDecryptionFailed
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		// Don't leak the underlying crypto error - just signal failure.
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}
