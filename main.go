package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go_onvif/db"
	"go_onvif/handler"
	"go_onvif/middleware"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	sqlite, err := db.Init("onvif.db")
	if err != nil {
		fmt.Fprintf(os.Stderr, "database init failed: %v\n", err)
		os.Exit(1)
	}
	defer sqlite.Close()
	fmt.Println("database: OK")

	dh := &handler.DeviceHandler{DB: sqlite}

	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/devices", dh.Create)
	mux.HandleFunc("GET /api/devices", dh.List)
	mux.HandleFunc("GET /api/devices/{id}", dh.Get)
	mux.HandleFunc("PUT /api/devices/{id}", dh.Update)
	mux.HandleFunc("DELETE /api/devices/{id}", dh.Delete)
	mux.HandleFunc("GET /api/devices/{id}/profiles", dh.Profiles)
	mux.HandleFunc("PUT /api/devices/{id}/stream-profile", dh.SetStreamProfile)

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		if err := sqlite.Ping(); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, `{"status":"unhealthy","error":"%s"}`, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	})

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      middleware.CORS(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		fmt.Printf("ias_media_server listening on :%s\n", port)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "server error: %v\n", err)
			os.Exit(1)
		}
	}()

	<-stop
	fmt.Println("\nshutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	server.Shutdown(ctx)
}
