package server

import (
	"context"
	"net/http"

	"github.com/godbus/dbus/v5"
)

const mmService = "org.freedesktop.ModemManager1"

type api struct {
	conn           *dbus.Conn
	push           *pushService
	debugPushToken string
}

type Server struct {
	handler http.Handler
	cancel  context.CancelFunc
}

type Config struct {
	StaticDir      string
	DataDir        string
	VAPIDSubject   string
	DebugPushToken string
}

func New(conn *dbus.Conn, config Config) (*Server, error) {
	push, err := newPushService(config.DataDir, config.VAPIDSubject)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	a := &api{conn: conn, push: push, debugPushToken: config.DebugPushToken}
	handler := routes(a, config.StaticDir)
	go a.watchIncomingMessages(ctx)
	return &Server{handler: handler, cancel: cancel}, nil
}

func (s *Server) Handler() http.Handler { return s.handler }
func (s *Server) Close()                { s.cancel() }

func NewHandler(conn *dbus.Conn, staticDir string) http.Handler {
	return routes(&api{conn: conn}, staticDir)
}

func routes(a *api, staticDir string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", a.health)
	mux.HandleFunc("GET /api/modems", a.modems)
	mux.HandleFunc("GET /api/messages", a.messages)
	mux.HandleFunc("POST /api/messages", a.sendMessage)
	mux.HandleFunc("DELETE /api/messages/{id}", a.deleteMessage)
	mux.HandleFunc("GET /api/push", a.pushConfig)
	mux.HandleFunc("POST /api/push/subscriptions", a.subscribePush)
	mux.HandleFunc("DELETE /api/push/subscriptions", a.unsubscribePush)
	mux.HandleFunc("GET /api/settings/push", a.getPushSettings)
	mux.HandleFunc("PUT /api/settings/push", a.updatePushSettings)
	mux.HandleFunc("POST /api/debug/push", a.debugPush)
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
