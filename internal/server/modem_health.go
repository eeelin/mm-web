package server

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	healthCheckInterval = 30 * time.Second
	healthFailureLimit  = 3
)

type modemHealthState struct {
	DBusAvailable bool     `json:"dbusAvailable"`
	ModemsOnline  bool     `json:"modemsOnline"`
	ATAvailable   bool     `json:"atAvailable"`
	ModemIDs      []string `json:"modemIds"`
	CheckedAt     string   `json:"checkedAt"`
}

type modemHealthMonitor struct {
	mu           sync.Mutex
	push         *pushService
	state        modemHealthState
	initialized  bool
	failures     map[string]int
	notifiedDown map[string]bool
}

func newModemHealthMonitor(push *pushService) *modemHealthMonitor {
	return &modemHealthMonitor{
		push:     push,
		state:    modemHealthState{DBusAvailable: true, ModemsOnline: true, ATAvailable: true},
		failures: make(map[string]int), notifiedDown: make(map[string]bool),
	}
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
	ticker := time.NewTicker(healthCheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			check()
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
		"url":   "/?screen=settings",
		"tag":   "modem-health-" + kind,
	}
}
