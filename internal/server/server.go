package server

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

const mmService = "org.freedesktop.ModemManager1"

type api struct {
	conn             *dbus.Conn
	push             *pushService
	debugPushToken   string
	startedAt        time.Time
	callSignalsOnce  sync.Once
	callSignalsErr   error
	callHistory      *callHistoryStore
	incomingNotified map[string]bool
	modemHealth      *modemHealthMonitor
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
	a := &api{conn: conn, push: push, debugPushToken: config.DebugPushToken, startedAt: time.Now(), incomingNotified: make(map[string]bool)}
	a.modemHealth = newModemHealthMonitor(push)
	a.callHistory, err = newCallHistoryStore(config.DataDir)
	if err != nil {
		cancel()
		return nil, err
	}
	handler := routes(a, config.StaticDir)
	go a.watchIncomingMessages(ctx)
	go a.watchCallHistory(ctx)
	go a.watchModemHealth(ctx)
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
	mux.HandleFunc("GET /api/calls", a.calls)
	mux.HandleFunc("GET /api/calls/events", a.callEvents)
	mux.HandleFunc("GET /api/call-history", a.getCallHistory)
	mux.HandleFunc("DELETE /api/call-history", a.clearCallHistory)
	mux.HandleFunc("POST /api/calls", a.createCall)
	mux.HandleFunc("POST /api/calls/{id}/hangup", a.hangupCall)
	mux.HandleFunc("POST /api/calls/{id}/accept", a.acceptCall)
	mux.HandleFunc("POST /api/calls/{id}/dtmf", a.sendCallDTMF)
	mux.HandleFunc("GET /api/push", a.pushConfig)
	mux.HandleFunc("POST /api/push/subscriptions", a.subscribePush)
	mux.HandleFunc("DELETE /api/push/subscriptions", a.unsubscribePush)
	mux.HandleFunc("GET /api/settings/push", a.getPushSettings)
	mux.HandleFunc("PUT /api/settings/push", a.updatePushSettings)
	mux.HandleFunc("GET /api/about", a.about)
	mux.HandleFunc("POST /api/debug/push", a.debugPush)
	mux.Handle("/", staticFiles(staticDir))
	return mux
}

func (a *api) health(w http.ResponseWriter, _ *http.Request) {
	var owner string
	err := a.conn.BusObject().Call("org.freedesktop.DBus.GetNameOwner", 0, mmService).Store(&owner)
	if err != nil {
		state := modemHealthState{CheckedAt: time.Now().UTC().Format(time.RFC3339)}
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "degraded", "error": "ModemManager 不可用", "modemManager": state})
		return
	}
	result := map[string]any{"status": "ok"}
	if a.modemHealth != nil {
		result["modemManager"] = a.modemHealth.snapshot()
	}
	writeJSON(w, http.StatusOK, result)
}
