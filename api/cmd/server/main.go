package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
	"tnr-atlas/api/internal/app"
)

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func main() {
	if len(os.Args) > 1 && os.Args[1] == "--healthcheck" {
		client := &http.Client{Timeout: 3 * time.Second}
		res, err := client.Get("http://127.0.0.1:8081/api/health")
		if err != nil {
			os.Exit(1)
		}
		res.Body.Close()
		if res.StatusCode != 200 {
			os.Exit(1)
		}
		return
	}
	code := os.Getenv("TNR_ACCESS_CODE")
	origin := os.Getenv("PUBLIC_ORIGIN")
	host := os.Getenv("ONEIMG_BASE_URL")
	if err := app.ValidateConfig(code, origin, host); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	imageOrigin := os.Getenv("ONEIMG_PUBLIC_ORIGIN")
	if err := app.ValidateImagePublicOrigin(imageOrigin); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	path := env("DATABASE_PATH", "data/tnr.db")
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		slog.Error("data directory unavailable")
		os.Exit(1)
	}
	store, err := app.Open(path)
	if err != nil {
		slog.Error("database unavailable")
		os.Exit(1)
	}
	defer store.Close()
	if len(os.Getenv("TNR_ADMIN_PASSWORD")) < 16 {
		slog.Error("TNR_ADMIN_PASSWORD must contain at least 16 characters")
		os.Exit(1)
	}
	if err = store.InitAuth(code); err != nil {
		slog.Error("auth settings unavailable")
		os.Exit(1)
	}
	api := app.NewServer(store, app.NewImageHost(host, os.Getenv("ONEIMG_TOKEN")), code, origin)
	api.Host.PublicOrigin = imageOrigin
	if raw := os.Getenv("ONEIMG_FOLDER_ID"); raw != "" {
		id, e := strconv.ParseInt(raw, 10, 64)
		if e != nil || id < 0 {
			slog.Error("ONEIMG_FOLDER_ID must be a nonnegative integer")
			os.Exit(1)
		}
		api.Host.FolderID = id
	}
	api.AdminPassword = os.Getenv("TNR_ADMIN_PASSWORD")
	srv := &http.Server{Addr: env("LISTEN_ADDR", ":8081"), Handler: app.Frontend(api.Handler(), env("FRONTEND_DIR", "../web/dist")), ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 2 * time.Minute, WriteTimeout: 5 * time.Minute, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown)
	}()
	slog.Info("TNR API started", "address", srv.Addr)
	if err = srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server stopped")
		os.Exit(1)
	}
}
