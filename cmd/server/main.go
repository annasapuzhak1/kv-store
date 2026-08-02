// Command server starts the kv-store HTTP server.
package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/annasapuzhak1/kv-store/internal/api"
	"github.com/annasapuzhak1/kv-store/internal/service"
	"github.com/annasapuzhak1/kv-store/internal/storage"
)

func main() {
	addr := ":8080"
	if v := os.Getenv("PORT"); v != "" {
		addr = ":" + v
	}

	store := storage.NewMemoryStore()
	svc := service.New(store)
	handler := api.NewHandler(svc)

	mux := http.NewServeMux()
	handler.Routes(mux)

	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("kv-store listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server failed: %v", err)
	}
}
