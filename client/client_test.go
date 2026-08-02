package client_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/annasapuzhak1/kv-store/client"
	"github.com/annasapuzhak1/kv-store/internal/api"
	"github.com/annasapuzhak1/kv-store/internal/service"
	"github.com/annasapuzhak1/kv-store/internal/storage"
)

// newTestServer spins up a real kv-store server (in-memory) so the client
// can be tested against actual HTTP round trips, not mocks.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	svc := service.New(storage.NewMemoryStore())
	h := api.NewHandler(svc)
	mux := http.NewServeMux()
	h.Routes(mux)
	return httptest.NewServer(mux)
}

func TestClient_StoreRetrieveUpdateDelete(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	c := client.New(server.URL)

	key, err := c.Store("id-1", []byte("hello"))
	if err != nil {
		t.Fatalf("Store() returned error: %v", err)
	}
	if key == "" {
		t.Fatal("Store() returned empty key")
	}

	data, err := c.Retrieve("id-1", key)
	if err != nil {
		t.Fatalf("Retrieve() returned error: %v", err)
	}
	if !bytes.Equal(data, []byte("hello")) {
		t.Fatalf("Retrieve() = %q, want %q", data, "hello")
	}

	if err := c.Update("id-1", key, []byte("updated")); err != nil {
		t.Fatalf("Update() returned error: %v", err)
	}

	data, err = c.Retrieve("id-1", key)
	if err != nil {
		t.Fatalf("Retrieve() after Update() returned error: %v", err)
	}
	if !bytes.Equal(data, []byte("updated")) {
		t.Fatalf("Retrieve() after Update() = %q, want %q", data, "updated")
	}

	if err := c.Delete("id-1", key); err != nil {
		t.Fatalf("Delete() returned error: %v", err)
	}

	_, err = c.Retrieve("id-1", key)
	if err == nil {
		t.Fatal("expected Retrieve() after Delete() to fail")
	}
	apiErr, ok := err.(*client.APIError)
	if !ok {
		t.Fatalf("expected *client.APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", apiErr.StatusCode)
	}
}

func TestClient_RetrieveWrongKey(t *testing.T) {
	server := newTestServer(t)
	defer server.Close()

	c := client.New(server.URL)
	_, err := c.Store("id-1", []byte("secret"))
	if err != nil {
		t.Fatalf("Store() returned error: %v", err)
	}

	_, err = c.Retrieve("id-1", "0000000000000000000000000000000000000000000000000000000000000000")
	if err == nil {
		t.Fatal("expected Retrieve() with wrong key to fail")
	}

	apiErr, ok := err.(*client.APIError)
	if !ok {
		t.Fatalf("expected *client.APIError, got %T", err)
	}

	if apiErr.StatusCode != http.StatusNotFound {
    	t.Fatalf("expected 404 for wrong key, got %d", apiErr.StatusCode)
  	}
}

func TestClient_StoreDuplicateID(t *testing.T) {
    server := newTestServer(t)
    defer server.Close()

    c := client.New(server.URL)
    _, err := c.Store("id-1", []byte("first"))
    if err != nil {
        t.Fatalf("Store() returned error: %v", err)
    }

    _, err = c.Store("id-1", []byte("second"))
    if err == nil {
        t.Fatal("expected Store() with duplicate id to fail")
    }
    apiErr, ok := err.(*client.APIError)
    if !ok {
        t.Fatalf("expected *client.APIError, got %T", err)
    }
    if apiErr.StatusCode != http.StatusConflict {
        t.Fatalf("expected 409 for duplicate id, got %d", apiErr.StatusCode)
    }
}
