package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/annasapuzhak1/kv-store/internal/service"
	"github.com/annasapuzhak1/kv-store/internal/storage"
)

func newTestServer() *httptest.Server {
	svc := service.New(storage.NewMemoryStore())
	h := NewHandler(svc)
	mux := http.NewServeMux()
	h.Routes(mux)
	return httptest.NewServer(mux)
}

func TestStoreAndRetrieve_HTTP(t *testing.T) {
	server := newTestServer()
	defer server.Close()

	// STORE
	storeBody, _ := json.Marshal(storeRequest{ID: "id-1", Data: "hello world"})
	resp, err := http.Post(server.URL+"/store", "application/json", bytes.NewReader(storeBody))
	if err != nil {
		t.Fatalf("POST /store failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("POST /store: got status %d, want %d", resp.StatusCode, http.StatusCreated)
	}

	var storeResp storeResponse
	if err := json.NewDecoder(resp.Body).Decode(&storeResp); err != nil {
		t.Fatalf("failed to decode store response: %v", err)
	}
	if storeResp.Key == "" {
		t.Fatal("expected non-empty key in store response")
	}

	// RETRIEVE
	retrieveResp, err := http.Get(server.URL + "/retrieve/id-1?key=" + storeResp.Key)
	if err != nil {
		t.Fatalf("GET /retrieve failed: %v", err)
	}
	defer retrieveResp.Body.Close()

	if retrieveResp.StatusCode != http.StatusOK {
		t.Fatalf("GET /retrieve: got status %d, want %d", retrieveResp.StatusCode, http.StatusOK)
	}

	var getResp retrieveResponse
	if err := json.NewDecoder(retrieveResp.Body).Decode(&getResp); err != nil {
		t.Fatalf("failed to decode retrieve response: %v", err)
	}
	if getResp.Data != "hello world" {
		t.Fatalf("GET /retrieve: got data %q, want %q", getResp.Data, "hello world")
	}
}

func TestRetrieve_WrongKey_HTTP(t *testing.T) {
	server := newTestServer()
	defer server.Close()

	storeBody, _ := json.Marshal(storeRequest{ID: "id-1", Data: "secret"})
	resp, _ := http.Post(server.URL+"/store", "application/json", bytes.NewReader(storeBody))
	resp.Body.Close()

	getResp, err := http.Get(server.URL + "/retrieve/id-1?key=0000000000000000000000000000000000000000000000000000000000000000")
	if err != nil {
		t.Fatalf("GET /retrieve failed: %v", err)
	}
	defer getResp.Body.Close()

	if getResp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET /retrieve with wrong key: got status %d, want %d", getResp.StatusCode, http.StatusNotFound)
	}
}

func TestRetrieve_NotFound_HTTP(t *testing.T) {
	server := newTestServer()
	defer server.Close()

	resp, err := http.Get(server.URL + "/retrieve/missing-id?key=0000000000000000000000000000000000000000000000000000000000000000")
	if err != nil {
		t.Fatalf("GET /retrieve failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET /retrieve on missing id: got status %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestUpdateAndDelete_HTTP(t *testing.T) {
	server := newTestServer()
	defer server.Close()

	storeBody, _ := json.Marshal(storeRequest{ID: "id-1", Data: "original"})
	resp, _ := http.Post(server.URL+"/store", "application/json", bytes.NewReader(storeBody))
	var storeResp storeResponse
	json.NewDecoder(resp.Body).Decode(&storeResp)
	resp.Body.Close()

	// UPDATE
	updateBody, _ := json.Marshal(updateRequest{Data: "updated"})
	req, _ := http.NewRequest(http.MethodPut, server.URL+"/update/id-1?key="+storeResp.Key, bytes.NewReader(updateBody))
	updateResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("PUT /update failed: %v", err)
	}
	updateResp.Body.Close()
	if updateResp.StatusCode != http.StatusNoContent {
		t.Fatalf("PUT /update: got status %d, want %d", updateResp.StatusCode, http.StatusNoContent)
	}

	// verify update took effect
	getResp, _ := http.Get(server.URL + "/retrieve/id-1?key=" + storeResp.Key)
	var getBody retrieveResponse
	json.NewDecoder(getResp.Body).Decode(&getBody)
	getResp.Body.Close()
	if getBody.Data != "updated" {
		t.Fatalf("after update, got data %q, want %q", getBody.Data, "updated")
	}

	// DELETE
	delReq, _ := http.NewRequest(http.MethodDelete, server.URL+"/delete/id-1?key="+storeResp.Key, nil)
	delResp, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatalf("DELETE /delete failed: %v", err)
	}
	delResp.Body.Close()
	if delResp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE /delete: got status %d, want %d", delResp.StatusCode, http.StatusNoContent)
	}

	// verify it's gone
	finalResp, _ := http.Get(server.URL + "/retrieve/id-1?key=" + storeResp.Key)
	finalResp.Body.Close()
	if finalResp.StatusCode != http.StatusNotFound {
		t.Fatalf("after delete, GET /retrieve: got status %d, want %d", finalResp.StatusCode, http.StatusNotFound)
	}
}
