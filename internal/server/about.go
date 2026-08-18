package server

import (
	"net/http"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/godbus/dbus/v5"
)

const Version = "0.2.0"

var Commit string

type aboutInfo struct {
	Version             string `json:"version"`
	Commit              string `json:"commit"`
	BuildTime           string `json:"buildTime"`
	GoVersion           string `json:"goVersion"`
	OS                  string `json:"os"`
	Arch                string `json:"arch"`
	UptimeSeconds       int64  `json:"uptimeSeconds"`
	ServerTime          string `json:"serverTime"`
	ModemManager        string `json:"modemManager"`
	ModemManagerVersion string `json:"modemManagerVersion"`
	PushSubscriptions   int    `json:"pushSubscriptions"`
	ShowMessageContent  bool   `json:"showMessageContent"`
}

func (a *api) about(w http.ResponseWriter, _ *http.Request) {
	info := currentBuildInfo()
	info.UptimeSeconds = int64(time.Since(a.startedAt).Seconds())
	info.ServerTime = time.Now().Format(time.RFC3339)
	info.ModemManager = "unavailable"
	var owner string
	if err := a.conn.BusObject().Call("org.freedesktop.DBus.GetNameOwner", 0, mmService).Store(&owner); err == nil {
		info.ModemManager = "connected"
		var version dbus.Variant
		if err := a.conn.Object(mmService, "/org/freedesktop/ModemManager1").Call("org.freedesktop.DBus.Properties.Get", 0, "org.freedesktop.ModemManager1", "Version").Store(&version); err == nil {
			info.ModemManagerVersion, _ = version.Value().(string)
		}
	}
	if a.push != nil {
		a.push.mu.Lock()
		info.PushSubscriptions = len(a.push.subscriptions)
		info.ShowMessageContent = a.push.settings.ShowMessageContent
		a.push.mu.Unlock()
	}
	writeJSON(w, http.StatusOK, info)
}

func currentBuildInfo() aboutInfo {
	info := aboutInfo{Version: Version, Commit: shortRevision(Commit), GoVersion: runtime.Version(), OS: runtime.GOOS, Arch: runtime.GOARCH}
	if build, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range build.Settings {
			switch setting.Key {
			case "vcs.revision":
				if info.Commit != "" {
					continue
				}
				info.Commit = shortRevision(setting.Value)
			case "vcs.time":
				info.BuildTime = setting.Value
			}
		}
	}
	return info
}

func shortRevision(value string) string {
	if len(value) > 8 {
		return value[:8]
	}
	return value
}
