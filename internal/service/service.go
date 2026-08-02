// Package service contains the business logic of the kv-store: it is the
// only package that knows about both encryption and storage, and it is
// responsible for the central design decision of this project - the
// service never persists an encryption key anywhere. Each key is generated,
// used once to encrypt, handed back to the caller, and then forgotten.
//
// It's also the layer that bridges the two packages' data shapes: it takes
// the (ciphertext, nonce) pair that encryption.Encrypt returns and builds
// the storage.Entry that storage.Store expects, and vice versa on the way
// back out.
package service

import (
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/annasapuzhak1/kv-store/internal/encryption"
	"github.com/annasapuzhak1/kv-store/internal/storage"
)

var (
	ErrEmptyID       = errors.New("service: id must not be empty")
	ErrNotFound      = errors.New("service: entry not found")
	ErrInvalidKey    = errors.New("service: invalid or malformed encryption key")
	ErrDecryptFailed = errors.New("service: decryption failed - wrong key or corrupted data")
	ErrAlreadyExists = errors.New("service: id already exists")
)

// Service implements the STORE / RETRIEVE / UPDATE / DELETE operations
// described in the task spec.
type Service struct {
	store storage.Store
}

// New creates a Service backed by the given storage implementation.
func New(store storage.Store) *Service {
	return &Service{store: store}
}

// Store encrypts data with a freshly generated key, persists the resulting
// ciphertext + nonce under id, and returns the key (hex-encoded) to the
// caller. The key is not kept anywhere after this call returns.
func (s *Service) Store(id string, data []byte) (keyHex string, err error) {
	if id == "" {
		return "", ErrEmptyID
	}

	key, err := encryption.GenerateKey()
	if err != nil {
		return "", fmt.Errorf("service: failed to generate key: %w", err)
	}

	ciphertext, nonce, err := encryption.Encrypt(data, key)
	if err != nil {
		return "", fmt.Errorf("service: failed to encrypt: %w", err)
	}

	entry := storage.Entry{Ciphertext: ciphertext, Nonce: nonce}
	if err := s.store.Create(id, entry); err != nil {
		if errors.Is(err, storage.ErrAlreadyExists) {
            return "", ErrAlreadyExists
        }
        return "", fmt.Errorf("service: failed to persist: %w", err)
	}

	return hex.EncodeToString(key), nil
}

// Retrieve fetches the entry stored under id and decrypts it using the
// caller-supplied key (hex-encoded). Returns the original plaintext.
func (s *Service) Retrieve(id, keyHex string) ([]byte, error) {
	if id == "" {
		return nil, ErrEmptyID
	}

	key, err := decodeKey(keyHex)
	if err != nil {
		return nil, err
	}

	entry, err := s.store.Get(id)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("service: failed to read: %w", err)
	}

	plaintext, err := encryption.Decrypt(entry.Ciphertext, entry.Nonce, key)
	if err != nil {
		return nil, ErrDecryptFailed
	}

	return plaintext, nil
}

// Update re-encrypts newData with the same key (a fresh nonce is generated
// each time). The caller must supply the correct key, which also proves the
// entry exists. The key does not change, per the spec.
//
// Known limitation: the key check and the write are not atomic - a
// concurrent request could modify the same id in between.
func (s *Service) Update(id, keyHex string, newData []byte) error {
	if id == "" {
		return ErrEmptyID
	}

	key, err := decodeKey(keyHex)
	if err != nil {
		return err
	}

	// Verify the caller holds the correct key before allowing an update -
	// otherwise anyone who merely knows/guesses an ID could overwrite data
	// they were never given the key to. This also confirms the id exists.
	if _, err := s.Retrieve(id, keyHex); err != nil {
		return err
	}

	ciphertext, nonce, err := encryption.Encrypt(newData, key)
	if err != nil {
		return fmt.Errorf("service: failed to encrypt: %w", err)
	}

	entry := storage.Entry{Ciphertext: ciphertext, Nonce: nonce}
	if err := s.store.Update(id, entry); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("service: failed to persist update: %w", err)
	}

	return nil
}

// Delete removes the entry stored under id. The caller must supply the
// correct key to prove they are allowed to delete it.
//
// Known limitation: the key check and the delete are not atomic - a
// concurrent request could modify the same id in between.
func (s *Service) Delete(id, keyHex string) error {
	if id == "" {
		return ErrEmptyID
	}

	if _, err := s.Retrieve(id, keyHex); err != nil {
		return err
	}

	if err := s.store.Delete(id); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return ErrNotFound
		}
		return fmt.Errorf("service: failed to delete: %w", err)
	}

	return nil
}

func decodeKey(keyHex string) ([]byte, error) {
	key, err := hex.DecodeString(keyHex)
	if err != nil || len(key) != encryption.KeySize {
		return nil, ErrInvalidKey
	}
	return key, nil
}
