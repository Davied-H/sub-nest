package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"sub-nest/internal/server"
	"sub-nest/internal/store"
)

func main() {
	addr := flag.String("addr", env("SUBAGG_ADDR", ":8080"), "listen address")
	dataPath := flag.String("data", env("SUBAGG_DATA", "data/config.json"), "config path")
	staticPath := flag.String("static", env("SUBAGG_STATIC", "web/dist"), "static frontend path")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	st, err := store.New(*dataPath)
	if err != nil {
		logger.Error("open store failed", "error", err)
		os.Exit(1)
	}
	app := server.New(st, *staticPath, logger)
	logger.Info("sub-nest listening", "addr", *addr, "data", *dataPath)
	if st.Snapshot().Settings.AdminTokenHash == "" {
		fmt.Printf("首次启动：打开后台后设置管理 token，或使用 API POST /api/setup。\n")
	}
	if err := http.ListenAndServe(*addr, app.Routes()); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func env(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
