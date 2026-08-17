package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/godbus/dbus/v5"
)

const modemInterface = "org.freedesktop.ModemManager1.Modem"

const (
	mmService          = "org.freedesktop.ModemManager1"
	messagingInterface = "org.freedesktop.ModemManager1.Modem.Messaging"
	smsInterface       = "org.freedesktop.ModemManager1.Sms"
)

var phoneNumberPattern = regexp.MustCompile(`^\+?[0-9]{3,20}$`)

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

type message struct {
	ID        string `json:"id"`
	ModemID   string `json:"modemId"`
	Number    string `json:"number"`
	Text      string `json:"text"`
	Direction string `json:"direction"`
	State     string `json:"state"`
	Timestamp string `json:"timestamp"`
}

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
	mux.HandleFunc("GET /api/messages", a.messages)
	mux.HandleFunc("POST /api/messages", a.sendMessage)
	mux.HandleFunc("DELETE /api/messages/{id}", a.deleteMessage)
	staticDir := fallback(os.Getenv("MM_WEB_STATIC_DIR"), "dist")
	mux.Handle("/", staticFiles(staticDir))

	addr := os.Getenv("MM_WEB_API_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 35 * time.Second}
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
	call := a.conn.Object(mmService, "/org/freedesktop/ModemManager1").Call(
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

func (a *api) messages(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	objects, err := a.managedObjects(ctx)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "读取短信失败", err)
		return
	}
	result, err := a.collectMessages(ctx, objects)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "读取短信内容失败", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": result, "updatedAt": time.Now().UTC()})
}

func (a *api) sendMessage(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ModemID string `json:"modemId"`
		Number  string `json:"number"`
		Text    string `json:"text"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式无效", err)
		return
	}
	input.Number = strings.TrimSpace(input.Number)
	input.Text = strings.TrimSpace(input.Text)
	if err := validateMessage(input.Number, input.Text); err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	objects, err := a.managedObjects(ctx)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "读取调制解调器失败", err)
		return
	}
	modemPath, err := selectMessagingModem(objects, input.ModemID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error(), err)
		return
	}
	properties := map[string]dbus.Variant{
		"number": dbus.MakeVariant(input.Number),
		"text":   dbus.MakeVariant(input.Text),
	}
	var smsPath dbus.ObjectPath
	if err := a.conn.Object(mmService, modemPath).CallWithContext(ctx, messagingInterface+".Create", 0, properties).Store(&smsPath); err != nil {
		writeMessagingError(w, "创建短信失败", err)
		return
	}
	if err := a.conn.Object(mmService, smsPath).CallWithContext(ctx, smsInterface+".Send", 0).Err; err != nil {
		writeMessagingError(w, "发送短信失败", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": smsID(smsPath), "status": "sent"})
}

func (a *api) deleteMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !digitsOnly(id) {
		writeError(w, http.StatusBadRequest, "短信编号无效", errors.New("invalid message id"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	objects, err := a.managedObjects(ctx)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "读取短信失败", err)
		return
	}
	smsPath := dbus.ObjectPath("/org/freedesktop/ModemManager1/SMS/" + id)
	modemPath, err := modemForSMS(objects, smsPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "短信不存在", err)
		return
	}
	if err := a.conn.Object(mmService, modemPath).CallWithContext(ctx, messagingInterface+".Delete", 0, smsPath).Err; err != nil {
		writeMessagingError(w, "删除短信失败", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) managedObjects(ctx context.Context) (map[dbus.ObjectPath]map[string]map[string]dbus.Variant, error) {
	var objects map[dbus.ObjectPath]map[string]map[string]dbus.Variant
	err := a.conn.Object(mmService, "/org/freedesktop/ModemManager1").CallWithContext(ctx, "org.freedesktop.DBus.ObjectManager.GetManagedObjects", 0).Store(&objects)
	return objects, err
}

func (a *api) collectMessages(ctx context.Context, objects map[dbus.ObjectPath]map[string]map[string]dbus.Variant) ([]message, error) {
	result := make([]message, 0)
	for modemPath, interfaces := range objects {
		messaging := interfaces[messagingInterface]
		if messaging == nil {
			continue
		}
		paths, _ := messaging["Messages"].Value().([]dbus.ObjectPath)
		for _, path := range paths {
			props := objects[path][smsInterface]
			if props == nil {
				props = make(map[string]dbus.Variant)
				if err := a.conn.Object(mmService, path).CallWithContext(ctx, "org.freedesktop.DBus.Properties.GetAll", 0, smsInterface).Store(&props); err != nil {
					return nil, err
				}
			}
			result = append(result, messageFromProps(path, modemPath, props))
		}
	}
	sort.Slice(result, func(i, j int) bool { return numericID(result[i].ID) > numericID(result[j].ID) })
	return result, nil
}

func messageFromProps(path, modemPath dbus.ObjectPath, props map[string]dbus.Variant) message {
	direction := "received"
	if number(props, "PduType") == 2 {
		direction = "sent"
	}
	return message{ID: smsID(path), ModemID: modemID(modemPath), Number: text(props, "Number"), Text: text(props, "Text"), Direction: direction, State: smsState(number(props, "State")), Timestamp: text(props, "Timestamp")}
}

func selectMessagingModem(objects map[dbus.ObjectPath]map[string]map[string]dbus.Variant, requested string) (dbus.ObjectPath, error) {
	paths := make([]dbus.ObjectPath, 0)
	for path, interfaces := range objects {
		if interfaces[messagingInterface] != nil && (requested == "" || modemID(path) == requested) {
			paths = append(paths, path)
		}
	}
	if len(paths) == 0 {
		return "", errors.New("未找到支持短信的调制解调器")
	}
	sort.Slice(paths, func(i, j int) bool { return paths[i] < paths[j] })
	return paths[0], nil
}

func modemForSMS(objects map[dbus.ObjectPath]map[string]map[string]dbus.Variant, smsPath dbus.ObjectPath) (dbus.ObjectPath, error) {
	for path, interfaces := range objects {
		messaging := interfaces[messagingInterface]
		if messaging == nil {
			continue
		}
		paths, _ := messaging["Messages"].Value().([]dbus.ObjectPath)
		for _, candidate := range paths {
			if candidate == smsPath {
				return path, nil
			}
		}
	}
	return "", errors.New("message not found")
}

func validateMessage(number, body string) error {
	if !phoneNumberPattern.MatchString(number) {
		return errors.New("请输入有效的电话号码")
	}
	length := utf8.RuneCountInString(body)
	if length == 0 {
		return errors.New("信息内容不能为空")
	}
	if length > 1600 {
		return errors.New("信息内容不能超过 1600 个字符")
	}
	return nil
}

func digitsOnly(value string) bool {
	return value != "" && strings.IndexFunc(value, func(r rune) bool { return r < '0' || r > '9' }) == -1
}
func smsID(path dbus.ObjectPath) string {
	return strings.TrimPrefix(string(path), "/org/freedesktop/ModemManager1/SMS/")
}
func modemID(path dbus.ObjectPath) string {
	return strings.TrimPrefix(string(path), "/org/freedesktop/ModemManager1/Modem/")
}
func numericID(id string) int { value, _ := strconv.Atoi(id); return value }
func smsState(state uint32) string {
	states := map[uint32]string{0: "unknown", 1: "stored", 2: "receiving", 3: "received", 4: "sending", 5: "sent"}
	return fallback(states[state], "unknown")
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

func writeMessagingError(w http.ResponseWriter, message string, err error) {
	if strings.Contains(err.Error(), "PolicyKit authorization failed") || strings.Contains(err.Error(), "not authorized") {
		writeError(w, http.StatusForbidden, "没有短信操作权限，请为 mm-web 配置 PolicyKit 授权", err)
		return
	}
	writeError(w, http.StatusBadGateway, message, err)
}
