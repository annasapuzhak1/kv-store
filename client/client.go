// Package client is the Go client other microservices import to talk to
// the kv-store over HTTP, without needing to know anything about the wire
// format themselves.
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// Client talks to a running kv-store server over HTTP.
type Client struct {
	baseURL string
	http    *http.Client
}

// New creates a Client pointed at the given kv-store base URL,
// e.g. "http://localhost:8080".
func New(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{},
	}
}

// APIError represents a non-2xx response from the kv-store server.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("kv-store client: server returned %d: %s", e.StatusCode, e.Message)
}

type storeRequest struct {
	ID   string `json:"id"`
	Data string `json:"data"`
}

type storeResponse struct {
	Key string `json:"key"`
}

type retrieveResponse struct {
	Data string `json:"data"`
}

type updateRequest struct {
	Data string `json:"data"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// Store inserts data under id and returns the encryption key. This key is
// never kept by the server - callers are responsible for storing it
// securely; it's required for every future Retrieve/Update/Delete call.
func (c *Client) Store(id string, data []byte) (key string, err error) {
	body, err := json.Marshal(storeRequest{ID: id, Data: string(data)})
	if err != nil {
		return "", fmt.Errorf("kv-store client: failed to marshal request: %w", err)
	}

	resp, err := c.http.Post(c.baseURL+"/store", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("kv-store client: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return "", apiErrorFrom(resp)
	}

	var out storeResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("kv-store client: failed to decode response: %w", err)
	}
	return out.Key, nil
}

// Retrieve fetches and decrypts the data stored under id, using the key
// returned from Store.
func (c *Client) Retrieve(id, key string) ([]byte, error) {
	u := fmt.Sprintf("%s/retrieve/%s?key=%s", c.baseURL, url.PathEscape(id), url.QueryEscape(key))

	resp, err := c.http.Get(u)
	if err != nil {
		return nil, fmt.Errorf("kv-store client: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, apiErrorFrom(resp)
	}

	var out retrieveResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("kv-store client: failed to decode response: %w", err)
	}
	return []byte(out.Data), nil
}

// Update replaces the data stored under id, re-encrypting it with the same
// key. The caller must supply the correct current key.
func (c *Client) Update(id, key string, newData []byte) error {
	body, err := json.Marshal(updateRequest{Data: string(newData)})
	if err != nil {
		return fmt.Errorf("kv-store client: failed to marshal request: %w", err)
	}

	u := fmt.Sprintf("%s/update/%s?key=%s", c.baseURL, url.PathEscape(id), url.QueryEscape(key))
	req, err := http.NewRequest(http.MethodPut, u, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("kv-store client: failed to build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("kv-store client: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return apiErrorFrom(resp)
	}
	return nil
}

// Delete removes the entry stored under id. The caller must supply the
// correct key.
func (c *Client) Delete(id, key string) error {
	u := fmt.Sprintf("%s/delete/%s?key=%s", c.baseURL, url.PathEscape(id), url.QueryEscape(key))
	req, err := http.NewRequest(http.MethodDelete, u, nil)
	if err != nil {
		return fmt.Errorf("kv-store client: failed to build request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("kv-store client: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return apiErrorFrom(resp)
	}
	return nil
}

func apiErrorFrom(resp *http.Response) error {
	var out errorResponse
	body, _ := io.ReadAll(resp.Body)
	message := string(body)
	if err := json.Unmarshal(body, &out); err == nil && out.Error != "" {
		message = out.Error
	}
	return &APIError{StatusCode: resp.StatusCode, Message: message}
}
