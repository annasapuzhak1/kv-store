package storage

import (
	"bytes"
	"sync"
	"testing"
)

func testEntry(data string) Entry {
	return Entry{
		Ciphertext: []byte(data),
		Nonce:      []byte("fake-nonce12"), // 12 bytes, storage doesn't care about real crypto
	}
}

func TestMemoryStore_CreateAndGet(t *testing.T) {
	s := NewMemoryStore()

	err := s.Create("id-1", testEntry("hello"))
	if err != nil {
		t.Fatalf("Create() returned error: %v", err)
	}

	got, err := s.Get("id-1")
	if err != nil {
		t.Fatalf("Get() returned error: %v", err)
	}
	if !bytes.Equal(got.Ciphertext, []byte("hello")) {
		t.Fatalf("Get().Ciphertext = %q, want %q", got.Ciphertext, "hello")
	}
	if !bytes.Equal(got.Nonce, []byte("fake-nonce12")) {
		t.Fatalf("Get().Nonce = %q, want %q", got.Nonce, "fake-nonce12")
	}
}

func TestMemoryStore_GetNotFound(t *testing.T) {
	s := NewMemoryStore()

	_, err := s.Get("missing")
	if err != ErrNotFound {
		t.Fatalf("Get() on missing id: got err %v, want ErrNotFound", err)
	}
}

func TestMemoryStore_Update(t *testing.T) {
	s := NewMemoryStore()
	_ = s.Create("id-1", testEntry("original"))

	err := s.Update("id-1", testEntry("updated"))
	if err != nil {
		t.Fatalf("Update() returned error: %v", err)
	}

	got, _ := s.Get("id-1")
	if !bytes.Equal(got.Ciphertext, []byte("updated")) {
		t.Fatalf("Get() after Update() = %q, want %q", got.Ciphertext, "updated")
	}
}

func TestMemoryStore_UpdateNotFound(t *testing.T) {
	s := NewMemoryStore()

	err := s.Update("missing", testEntry("data"))
	if err != ErrNotFound {
		t.Fatalf("Update() on missing id: got err %v, want ErrNotFound", err)
	}
}

func TestMemoryStore_Delete(t *testing.T) {
	s := NewMemoryStore()
	_ = s.Create("id-1", testEntry("data"))

	err := s.Delete("id-1")
	if err != nil {
		t.Fatalf("Delete() returned error: %v", err)
	}

	_, err = s.Get("id-1")
	if err != ErrNotFound {
		t.Fatalf("Get() after Delete(): got err %v, want ErrNotFound", err)
	}
}

func TestMemoryStore_DeleteNotFound(t *testing.T) {
	s := NewMemoryStore()

	err := s.Delete("missing")
	if err != ErrNotFound {
		t.Fatalf("Delete() on missing id: got err %v, want ErrNotFound", err)
	}
}

func TestMemoryStore_ConcurrentAccess(t *testing.T) {
	s := NewMemoryStore()
	var wg sync.WaitGroup

	// Hammer the store with concurrent writes/reads to catch data races.
	// Run with `go test -race` to actually verify this.
	for i := 0; i < 100; i++ {
		wg.Add(2)
		id := "id-1"
		go func() {
			defer wg.Done()
			_ = s.Create(id, testEntry("data"))
		}()
		go func() {
			defer wg.Done()
			_, _ = s.Get(id)
		}()
	}
	wg.Wait()
}
