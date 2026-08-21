package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	defaultHealthInterval = 2 * time.Minute
	healthFailureLimit    = 3
)

type modemHealthState struct {
	DBusAvailable bool     `json:"dbusAvailable"`
	ModemsOnline  bool     `json:"modemsOnline"`
	ATAvailable   bool     `json:"atAvailable"`
	ModemIDs      []string `json:"modemIds"`
	CheckedAt     string   `json:"checkedAt"`
}

type modemHealthMonitor struct {
	mu              sync.Mutex
	push            *pushService
	state           modemHealthState
	initialized     bool
	failures        map[string]int
	notifiedDown    map[string]bool
	dir             string
	settings        healthSettings
	settingsChanged chan struct{}
}

type healthSettings struct {
	CheckIntervalSeconds int `json:"checkIntervalSeconds"`
}

func newModemHealthMonitor(push *pushService, dirs ...string) *modemHealthMonitor {
	monitor := &modemHealthMonitor{
		push:     push,
		state:    modemHealthState{DBusAvailable: true, ModemsOnline: true, ATAvailable: true},
		failures: make(map[string]int), notifiedDown: make(map[string]bool),
		settings:        healthSettings{CheckIntervalSeconds: int(defaultHealthInterval.Seconds())},
		settingsChanged: make(chan struct{}, 1),
	}
	if len(dirs) > 0 {
		monitor.dir = dirs[0]
		var stored healthSettings
		if err := readJSON(filepath.Join(monitor.dir, "health-settings.json"), &stored); err == nil && validHealthInterval(stored.CheckIntervalSeconds) {
			monitor.settings = stored
		} else if err != nil && !os.IsNotExist(err) {
			log.Printf("load health settings: %v", err)
		}
	}
	return monitor
}

func validHealthInterval(seconds int) bool { return seconds >= 30 && seconds <= 3600 }

func (m *modemHealthMonitor) getSettings() healthSettings {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.settings
}

func (m *modemHealthMonitor) updateSettings(settings healthSettings) error {
	if !validHealthInterval(settings.CheckIntervalSeconds) {
		return fmt.Errorf("check interval must be between 30 and 3600 seconds")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.dir != "" {
		if err := writeJSONFile(filepath.Join(m.dir, "health-settings.json"), settings); err != nil {
			return err
		}
	}
	m.settings = settings
	select {
	case m.settingsChanged <- struct{}{}:
	default:
	}
	return nil
}

func (a *api) getHealthSettings(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, a.modemHealth.getSettings())
}

func (a *api) updateHealthSettings(w http.ResponseWriter, r *http.Request) {
	var settings healthSettings
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&settings); err != nil {
		writeError(w, http.StatusBadRequest, "检测设置格式无效", err)
		return
	}
	if err := a.modemHealth.updateSettings(settings); err != nil {
		writeError(w, http.StatusBadRequest, "检测间隔无效", err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (m *modemHealthMonitor) snapshot() modemHealthState {
	m.mu.Lock()
	defer m.mu.Unlock()
	state := m.state
	state.ModemIDs = append([]string(nil), state.ModemIDs...)
	return state
}

func (a *api) watchModemHealth(ctx context.Context) {
	check := func() {
		probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		a.modemHealth.observe(a.probeModemHealth(probeCtx))
	}
	check()
	for {
		timer := time.NewTimer(time.Duration(a.modemHealth.getSettings().CheckIntervalSeconds) * time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			check()
		case <-a.modemHealth.settingsChanged:
			timer.Stop()
		}
	}
}

func (a *api) probeModemHealth(ctx context.Context) modemHealthState {
	state := modemHealthState{DBusAvailable: true, CheckedAt: time.Now().UTC().Format(time.RFC3339)}
	objects, err := a.managedObjects(ctx)
	if err != nil {
		state.DBusAvailable = false
		return state
	}
	state.ATAvailable = true
	for path, interfaces := range objects {
		props := interfaces[modemInterface]
		if props == nil {
			continue
		}
		state.ModemIDs = append(state.ModemIDs, modemID(path))
		if modemHasATPort(props) && !signalQualityRecent(props["SignalQuality"]) {
			state.ATAvailable = false
		}
	}
	sort.Strings(state.ModemIDs)
	state.ModemsOnline = len(state.ModemIDs) > 0
	if !state.ModemsOnline {
		state.ATAvailable = false
	}
	return state
}

func modemHasATPort(props map[string]dbus.Variant) bool {
	ports, _ := props["Ports"].Value().([][]interface{})
	for _, port := range ports {
		if len(port) > 1 {
			kind, _ := port[1].(uint32)
			if kind == 3 {
				return true
			}
		}
	}
	return false
}

func signalQualityRecent(value dbus.Variant) bool {
	values, _ := value.Value().([]interface{})
	if len(values) < 2 {
		return false
	}
	recent, _ := values[1].(bool)
	return recent
}

func (m *modemHealthMonitor) observe(state modemHealthState) {
	m.mu.Lock()
	m.state = state
	checks := map[string]bool{
		"dbus": state.DBusAvailable,
		// Only notify the highest actionable failure. A D-Bus outage also makes
		// modem and AT checks impossible, but should not generate three pushes.
		"modem": !state.DBusAvailable || state.ModemsOnline,
		"at":    !state.DBusAvailable || !state.ModemsOnline || state.ATAvailable,
	}
	var alerts []string
	for key, healthy := range checks {
		if healthy {
			m.failures[key] = 0
			m.notifiedDown[key] = false
			continue
		}
		m.failures[key]++
		if m.initialized && m.failures[key] >= healthFailureLimit && !m.notifiedDown[key] {
			m.notifiedDown[key] = true
			alerts = append(alerts, key)
		}
	}
	m.initialized = true
	m.mu.Unlock()

	for _, key := range alerts {
		log.Printf("modem health alert: %s unavailable", key)
		if m.push != nil {
			go m.push.notifyModemHealthFailure(key)
		}
	}
}

func modemHealthPushContent(kind string) map[string]string {
	bodies := map[string]string{
		"dbus":  "无法连接 ModemManager，电话、信息和设备状态可能不可用。",
		"modem": "未检测到蜂窝调制解调器，请检查设备连接和供电。",
		"at":    "AT 控制通道连续无响应，短信、电话和信号刷新可能不可用。",
	}
	return map[string]string{
		"title": "蜂窝调制解调器异常",
		"body":  fmt.Sprintf("%s", fallback(bodies[kind], "检测到调制解调器异常。")),
		"url":   "/?screen=diagnostics",
		"tag":   "modem-health-" + kind,
	}
}
