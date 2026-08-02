package service

import (
	"bytes"
	"testing"

	"github.com/annasapuzhak1/kv-store/internal/storage"
)

func newTestService() *Service {
	return New(storage.NewMemoryStore())
}

func TestStoreAndRetrieve(t *testing.T) {
	svc := newTestService()

	key, err := svc.Store("id-1", []byte("odometer: 45213"))
	if err != nil {
		t.Fatalf("Store() returned error: %v", err)
	}
	if key == "" {
		t.Fatal("Store() returned empty key")
	}

	data, err := svc.Retrieve("id-1", key)
	if err != nil {
		t.Fatalf("Retrieve() returned error: %v", err)
	}
	if !bytes.Equal(data, []byte("odometer: 45213")) {
		t.Fatalf("Retrieve() = %q, want %q", data, "odometer: 45213")
	}
}

func TestRetrieve_WrongKeyFails(t *testing.T) {
	svc := newTestService()

	_, err := svc.Store("id-1", []byte("secret"))
	if err != nil {
		t.Fatalf("Store() returned error: %v", err)
	}

	otherKey, _ := svc.Store("id-2", []byte("other secret")) // valid key, wrong entry

	_, err = svc.Retrieve("id-1", otherKey)
	if err != ErrDecryptFailed {
		t.Fatalf("Retrieve() with wrong key: got err %v, want ErrDecryptFailed", err)
	}
}

func TestRetrieve_NotFound(t *testing.T) {
	svc := newTestService()

	validShapedKey := "0000000000000000000000000000000000000000000000000000000000000000"

	_, err := svc.Retrieve("missing-id", validShapedKey)
	if err != ErrNotFound {
		t.Fatalf("Retrieve() on missing id: got err %v, want ErrNotFound", err)
	}
}

func TestRetrieve_MalformedKey(t *testing.T) {
	svc := newTestService()
	_, _ = svc.Store("id-1", []byte("data"))

	_, err := svc.Retrieve("id-1", "not-valid-hex!!")
	if err != ErrInvalidKey {
		t.Fatalf("Retrieve() with malformed key: got err %v, want ErrInvalidKey", err)
	}
}

func TestUpdate(t *testing.T) {
	svc := newTestService()

	key, _ := svc.Store("id-1", []byte("original data"))

	err := svc.Update("id-1", key, []byte("updated data"))
	if err != nil {
		t.Fatalf("Update() returned error: %v", err)
	}

	// Same key should still work to retrieve the new data - Update's spec
	// output is "none", meaning the key does not change.
	data, err := svc.Retrieve("id-1", key)
	if err != nil {
		t.Fatalf("Retrieve() after Update() returned error: %v", err)
	}
	if !bytes.Equal(data, []byte("updated data")) {
		t.Fatalf("Retrieve() after Update() = %q, want %q", data, "updated data")
	}
}

func TestUpdate_WrongKeyRejected(t *testing.T) {
	svc := newTestService()
	_, _ = svc.Store("id-1", []byte("original"))
	wrongKey, _ := svc.Store("id-2", []byte("unrelated"))

	err := svc.Update("id-1", wrongKey, []byte("attempted overwrite"))
	if err != ErrDecryptFailed {
		t.Fatalf("Update() with wrong key: got err %v, want ErrDecryptFailed", err)
	}
}

func TestUpdate_NotFound(t *testing.T) {
	svc := newTestService()

	validShapedKey := "0000000000000000000000000000000000000000000000000000000000000000"

	err := svc.Update("missing-id", validShapedKey, []byte("data"))
	if err == nil {
		t.Fatal("expected Update() on missing id to fail")
	}
}

func TestDelete(t *testing.T) {
	svc := newTestService()
	key, _ := svc.Store("id-1", []byte("data"))

	err := svc.Delete("id-1", key)
	if err != nil {
		t.Fatalf("Delete() returned error: %v", err)
	}

	_, err = svc.Retrieve("id-1", key)
	if err != ErrNotFound {
		t.Fatalf("Retrieve() after Delete(): got err %v, want ErrNotFound", err)
	}
}

func TestDelete_WrongKeyRejected(t *testing.T) {
	svc := newTestService()
	_, _ = svc.Store("id-1", []byte("data"))
	wrongKey, _ := svc.Store("id-2", []byte("unrelated"))

	err := svc.Delete("id-1", wrongKey)
	if err != ErrDecryptFailed {
		t.Fatalf("Delete() with wrong key: got err %v, want ErrDecryptFailed", err)
	}
}

func TestStore_EmptyIDRejected(t *testing.T) {
	svc := newTestService()

	_, err := svc.Store("", []byte("data"))
	if err != ErrEmptyID {
		t.Fatalf("Store() with empty id: got err %v, want ErrEmptyID", err)
	}
}
