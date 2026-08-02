// Package storage provides the persistence layer for the kv-store.
//
// It knows nothing about *how* encryption works - it just holds an Entry
// (ciphertext + nonce) per ID. The Store interface is defined so a
// different implementation (e.g. bbolt, Postgres) could be swapped in
// later without changing any other package.
package storage

import (
	"errors"
	"sync"
)

var (
	ErrNotFound      = errors.New("entry not found")
	ErrAlreadyExists = errors.New("entry already exists")
)

// Entry is the record stored per ID: the encrypted data plus the nonce
// that was used to encrypt it. The nonce is not secret - GCM only requires
// that a (key, nonce) pair is never reused, not that the nonce is hidden -
// so it's safe to keep alongside the ciphertext rather than separately.
type Entry struct {
	Ciphertext []byte
	Nonce      []byte
}

// Store is the persistence interface the rest of the app depends on.
// Any implementation (in-memory, bbolt, SQL, ...) must satisfy this.
type Store interface {
	Create(id string, entry Entry) error
	Get(id string) (Entry, error)
	Update(id string, entry Entry) error
	Delete(id string) error
}

// MemoryStore is a simple in-memory, concurrency-safe implementation of Store.
// Data does not survive a process restart - this is a deliberate simplicity
// tradeoff for this task; see README for discussion.
type MemoryStore struct {
	mu   sync.RWMutex
	data map[string]Entry
}

// NewMemoryStore creates an empty, ready-to-use in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		data: make(map[string]Entry),
	}
}

// Create inserts entry under id. Returns ErrAlreadyExists if id is already in use.
func (s *MemoryStore) Create(id string, entry Entry) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    if _, ok := s.data[id]; ok {
        return ErrAlreadyExists
    }

    s.data[id] = copyEntry(entry)
    return nil
}

// Get retrieves the entry stored under id, or ErrNotFound.
func (s *MemoryStore) Get(id string) (Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.data[id]
	if !ok {
		return Entry{}, ErrNotFound
	}

	return copyEntry(entry), nil
}

// Update replaces the entry stored under id. Returns ErrNotFound if id
// doesn't already exist - Update is not an upsert.
func (s *MemoryStore) Update(id string, entry Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data[id]; !ok {
		return ErrNotFound
	}

	s.data[id] = copyEntry(entry)
	return nil
}

// Delete removes the entry stored under id. Returns ErrNotFound if it
// doesn't exist.
func (s *MemoryStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data[id]; !ok {
		return ErrNotFound
	}
	delete(s.data, id)
	return nil
}

// copyEntry returns a deep copy of entry so callers can't mutate our
// internal state through a slice they still hold a reference to.
func copyEntry(entry Entry) Entry {
	ct := make([]byte, len(entry.Ciphertext))
	copy(ct, entry.Ciphertext)
	n := make([]byte, len(entry.Nonce))
	copy(n, entry.Nonce)
	return Entry{Ciphertext: ct, Nonce: n}
}
