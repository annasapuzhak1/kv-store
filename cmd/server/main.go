// Command server starts the kv-store HTTP server.
package main

import (
	"log"
	"net/http"
	"os"

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

	log.Printf("kv-store listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
