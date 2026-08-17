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
	app, err := server.New(conn, server.Config{
		StaticDir:      envOr("MM_WEB_STATIC_DIR", "dist"),
		DataDir:        envOr("MM_WEB_DATA_DIR", "data"),
		VAPIDSubject:   envOr("MM_WEB_VAPID_SUBJECT", "mailto:admin@example.com"),
		DebugPushToken: os.Getenv("MM_WEB_DEBUG_PUSH_TOKEN"),
	})
	if err != nil {
		log.Fatalf("initialize server: %v", err)
	}
	defer app.Close()
	httpServer := &http.Server{Addr: addr, Handler: app.Handler(), ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 35 * time.Second}
	log.Printf("mm-web API listening on http://%s", addr)
	log.Fatal(httpServer.ListenAndServe())
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
