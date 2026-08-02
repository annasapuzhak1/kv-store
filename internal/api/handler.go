// Package api translates HTTP requests into calls against the service
// layer, and service results back into HTTP responses. It has no business
// logic of its own and never talks to storage or encryption directly.
package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/annasapuzhak1/kv-store/internal/service"
)

// Handler holds the dependencies needed to serve HTTP requests.
type Handler struct {
	svc *service.Service
}

// NewHandler creates a Handler backed by the given service.
func NewHandler(svc *service.Service) *Handler {
	return &Handler{svc: svc}
}

// Routes registers all kv-store endpoints on the given mux.
func (h *Handler) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /store", h.handleStore)
	mux.HandleFunc("GET /retrieve/{id}", h.handleRetrieve)
	mux.HandleFunc("PUT /update/{id}", h.handleUpdate)
	mux.HandleFunc("DELETE /delete/{id}", h.handleDelete)
}

// ---- request/response types ----

type storeRequest struct {
	ID   string `json:"id"`
	Data string `json:"data"` // treated as raw bytes of the given string
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

// ---- handlers ----

func (h *Handler) handleStore(w http.ResponseWriter, r *http.Request) {
	var req storeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	key, err := h.svc.Store(req.ID, []byte(req.Data))
	if err != nil {
		writeServiceError(w, r, err)
		return
	}

	writeJSON(w, http.StatusCreated, storeResponse{Key: key})
}

func (h *Handler) handleRetrieve(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	key := r.URL.Query().Get("key")

	data, err := h.svc.Retrieve(id, key)
	if err != nil {
		writeServiceError(w, r, err)
		return
	}

	writeJSON(w, http.StatusOK, retrieveResponse{Data: string(data)})
}

func (h *Handler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	key := r.URL.Query().Get("key")

	var req updateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if err := h.svc.Update(id, key, []byte(req.Data)); err != nil {
		writeServiceError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	key := r.URL.Query().Get("key")

	if err := h.svc.Delete(id, key); err != nil {
		writeServiceError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ---- helpers ----

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, errorResponse{Error: message})
}

// writeServiceError maps known service-layer errors to appropriate HTTP
// status codes, so callers of the API get meaningful responses instead of
// a generic 500 for everything.
// api/handler.go
func writeServiceError(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("%s %s -> error: %v", r.Method, r.URL.Path, err)

	switch {
	case errors.Is(err, service.ErrEmptyID):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrInvalidKey):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, service.ErrNotFound), errors.Is(err, service.ErrDecryptFailed):
		writeError(w, http.StatusNotFound, "not found")
	case errors.Is(err, service.ErrAlreadyExists):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}
