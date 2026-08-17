package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

const modemInterface = "org.freedesktop.ModemManager1.Modem"

type modem struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Model        string `json:"model"`
	State        string `json:"state"`
	Network      string `json:"network"`
	Tech         string `json:"tech"`
	Signal       uint32 `json:"signal"`
	SIM          string `json:"sim"`
	IMEI         string `json:"imei"`
	Firmware     string `json:"firmware"`
	Port         string `json:"port"`
	Manufacturer string `json:"manufacturer"`
}

type api struct{ conn *dbus.Conn }

func main() {
	conn, err := dbus.ConnectSystemBus()
	if err != nil {
		log.Fatalf("connect to system D-Bus: %v", err)
	}
	defer conn.Close()

	a := &api{conn: conn}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", a.health)
	mux.HandleFunc("GET /api/modems", a.modems)
	staticDir := fallback(os.Getenv("MM_WEB_STATIC_DIR"), "dist")
	mux.Handle("/", staticFiles(staticDir))

	addr := os.Getenv("MM_WEB_API_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.Printf("mm-web API listening on http://%s", addr)
	log.Fatal(server.ListenAndServe())
}

func staticFiles(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(dir, filepath.Clean(r.URL.Path))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			files.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(dir, "index.html"))
	})
}

func (a *api) health(w http.ResponseWriter, _ *http.Request) {
	var owner string
	err := a.conn.BusObject().Call("org.freedesktop.DBus.GetNameOwner", 0, "org.freedesktop.ModemManager1").Store(&owner)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "ModemManager 不可用", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *api) modems(w http.ResponseWriter, _ *http.Request) {
	var objects map[dbus.ObjectPath]map[string]map[string]dbus.Variant
	call := a.conn.Object("org.freedesktop.ModemManager1", "/org/freedesktop/ModemManager1").Call(
		"org.freedesktop.DBus.ObjectManager.GetManagedObjects", 0,
	)
	if err := call.Store(&objects); err != nil {
		writeError(w, http.StatusServiceUnavailable, "读取 ModemManager 失败", err)
		return
	}

	result := make([]modem, 0)
	for path, interfaces := range objects {
		props, ok := interfaces[modemInterface]
		if !ok {
			continue
		}
		m := modem{
			ID:           strings.TrimPrefix(string(path), "/org/freedesktop/ModemManager1/Modem/"),
			Name:         joinName(text(props, "Manufacturer"), text(props, "Model")),
			Model:        fallback(text(props, "Model"), "未知型号"),
			State:        modemState(number(props, "State")),
			Network:      "未注册网络",
			Tech:         accessTechnology(number(props, "AccessTechnologies")),
			Signal:       signalQuality(props["SignalQuality"]),
			SIM:          "未插入 SIM 卡",
			IMEI:         text(props, "EquipmentIdentifier"),
			Firmware:     text(props, "Revision"),
			Port:         text(props, "PrimaryPort"),
			Manufacturer: text(props, "Manufacturer"),
		}
		if p := interfaces["org.freedesktop.ModemManager1.Modem.Modem3gpp"]; p != nil {
			m.Network = fallback(text(p, "OperatorName"), m.Network)
		}
		if simPath, ok := props["Sim"].Value().(dbus.ObjectPath); ok && simPath.IsValid() && simPath != "/" {
			m.SIM = "SIM · 就绪"
			if p := objects[simPath]["org.freedesktop.ModemManager1.Sim"]; p != nil {
				operator := text(p, "OperatorName")
				if operator != "" && m.Network == "未注册网络" {
					m.Network = operator
				}
			}
		}
		result = append(result, m)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	writeJSON(w, http.StatusOK, map[string]any{"modems": result, "updatedAt": time.Now().UTC()})
}

func text(props map[string]dbus.Variant, key string) string {
	v, ok := props[key]
	if !ok {
		return ""
	}
	s, _ := v.Value().(string)
	return s
}

func number(props map[string]dbus.Variant, key string) uint32 {
	v, ok := props[key]
	if !ok {
		return 0
	}
	switch n := v.Value().(type) {
	case uint32:
		return n
	case int32:
		return uint32(n)
	default:
		return 0
	}
}

func signalQuality(v dbus.Variant) uint32 {
	values, ok := v.Value().([]interface{})
	if ok && len(values) > 0 {
		if n, ok := values[0].(uint32); ok {
			return n
		}
	}
	return 0
}

func modemState(state uint32) string {
	states := map[uint32]string{3: "已禁用", 4: "正在禁用", 5: "正在启用", 6: "已启用", 7: "正在搜索网络", 8: "已注册", 9: "正在断开", 10: "正在连接", 11: "已连接"}
	return fallback(states[state], "未知状态")
}

func accessTechnology(mask uint32) string {
	types := []struct {
		bit  uint32
		name string
	}{{1 << 15, "5G NR"}, {1 << 14, "4G LTE"}, {1 << 11, "HSPA+"}, {1 << 9, "HSPA"}, {1 << 7, "3G UMTS"}, {1 << 5, "EDGE"}, {1 << 4, "GPRS"}, {1 << 1, "GSM"}}
	var names []string
	for _, t := range types {
		if mask&t.bit != 0 {
			names = append(names, t.name)
		}
	}
	if len(names) == 0 {
		return "未知制式"
	}
	return strings.Join(names, " / ")
}

func joinName(manufacturer, model string) string {
	name := strings.TrimSpace(manufacturer + " " + model)
	return fallback(name, "调制解调器")
}

func fallback(value, defaultValue string) string {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("encode response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string, err error) {
	if err == nil {
		err = errors.New(strconv.Itoa(status))
	}
	log.Printf("%s: %v", message, err)
	writeJSON(w, status, map[string]string{"error": message, "detail": err.Error()})
}
