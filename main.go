package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"go_onvif/config"
	"go_onvif/db"
	"go_onvif/handler"
	"go_onvif/middleware"
	"go_onvif/onvif"
	"go_onvif/stream"
)

func main() {
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(h))

	godotenv.Load()

	cfg := config.Load()

	sqlite, err := db.Init(cfg.DBPath)
	if err != nil {
		slog.Error("database init failed", "error", err)
		os.Exit(1)
	}
	defer sqlite.Close()
	slog.Info("database initialized", "path", cfg.DBPath)

	streamMgr, err := stream.NewStreamManager(cfg.HLSDir, cfg.HLSTime, cfg.HLSListSize)
	if err != nil {
		slog.Error("stream manager init failed", "error", err)
		os.Exit(1)
	}
	defer streamMgr.Shutdown()
	slog.Info("stream manager initialized", "hls_dir", cfg.HLSDir, "hls_time", cfg.HLSTime, "hls_list_size", cfg.HLSListSize)

	dh := &handler.DeviceHandler{DB: sqlite, StreamMgr: streamMgr}
	sh := &handler.StreamHandler{Manager: streamMgr, DB: sqlite}

	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/devices", dh.Create)
	mux.HandleFunc("GET /api/devices", dh.List)
	mux.HandleFunc("GET /api/devices/{id}", dh.Get)
	mux.HandleFunc("PUT /api/devices/{id}", dh.Update)
	mux.HandleFunc("DELETE /api/devices/{id}", dh.Delete)
	mux.HandleFunc("GET /api/devices/{id}/profiles", dh.Profiles)
	mux.HandleFunc("PUT /api/devices/{id}/stream-profile", dh.SetStreamProfile)

	mux.HandleFunc("GET /api/streams", sh.List)
	mux.HandleFunc("POST /api/streams/{device_id}/start", sh.Start)
	mux.HandleFunc("POST /api/streams/{device_id}/stop", sh.Stop)

	mux.Handle("GET /hls/", http.StripPrefix("/hls/", http.FileServer(http.Dir(cfg.HLSDir))))

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
		Addr:         ":" + cfg.Port,
		Handler:      middleware.CORS(mux),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		slog.Info("server listening", "port", cfg.Port)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	go autoIngest(sqlite, streamMgr)

	<-stop
	slog.Info("shutting down")

	streamMgr.Shutdown()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	server.Shutdown(ctx)
}

func autoIngest(database *sql.DB, mgr *stream.StreamManager) {
	time.Sleep(500 * time.Millisecond)

	devices, err := db.ListDevices(database)
	if err != nil {
		slog.Error("auto-ingest: list devices failed", "error", err)
		return
	}

	for _, d := range devices {
		if d.StreamProfileToken == "" {
			continue
		}
		dev := d
		go func() {
			client, err := onvif.Connect(dev.IP, dev.Port, dev.Username, dev.Password)
			if err != nil {
				slog.Warn("auto-ingest: onvif connect failed", "device_id", dev.ID, "device_name", dev.Name, "error", err)
				return
			}
			rtspURL, err := client.GetStreamUri(dev.StreamProfileToken)
			if err != nil {
				slog.Warn("auto-ingest: GetStreamUri failed", "device_id", dev.ID, "device_name", dev.Name, "error", err)
				return
			}
			mgr.StartStream(dev.ID, dev.Name, dev.StreamProfileToken, rtspURL)
		}()
	}
}
