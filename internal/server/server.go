package server

import (
	"net/http"

	"github.com/godbus/dbus/v5"
)

const mmService = "org.freedesktop.ModemManager1"

type api struct{ conn *dbus.Conn }

func NewHandler(conn *dbus.Conn, staticDir string) http.Handler {
	a := &api{conn: conn}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", a.health)
	mux.HandleFunc("GET /api/modems", a.modems)
	mux.HandleFunc("GET /api/messages", a.messages)
	mux.HandleFunc("POST /api/messages", a.sendMessage)
	mux.HandleFunc("DELETE /api/messages/{id}", a.deleteMessage)
	mux.Handle("/", staticFiles(staticDir))
	return mux
}

func (a *api) health(w http.ResponseWriter, _ *http.Request) {
	var owner string
	err := a.conn.BusObject().Call("org.freedesktop.DBus.GetNameOwner", 0, mmService).Store(&owner)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "ModemManager 不可用", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
