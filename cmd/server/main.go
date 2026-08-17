package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/eeelin/mm-web/internal/server"
	"github.com/godbus/dbus/v5"
)

func main() {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		log.Fatalf("connect to system D-Bus: %v", err)
	}
	defer conn.Close()

	addr := envOr("MM_WEB_API_ADDR", ":8080")
	handler := server.NewHandler(conn, envOr("MM_WEB_STATIC_DIR", "dist"))
	httpServer := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 35 * time.Second}
	log.Printf("mm-web API listening on http://%s", addr)
	log.Fatal(httpServer.ListenAndServe())
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
