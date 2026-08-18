package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	voiceInterface = "org.freedesktop.ModemManager1.Modem.Voice"
	callInterface  = "org.freedesktop.ModemManager1.Call"
)

var callNumberPattern = regexp.MustCompile(`^\+?[0-9*#]{1,32}$`)
var dtmfPattern = regexp.MustCompile(`^[0-9*#]+$`)

type voiceCall struct {
	ID        string `json:"id"`
	ModemID   string `json:"modemId"`
	Number    string `json:"number"`
	Direction string `json:"direction"`
	State     string `json:"state"`
	Reason    string `json:"reason,omitempty"`
}

func (a *api) calls(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	objects, err := a.managedObjects(ctx)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "读取电话状态失败", err)
		return
	}
	calls, err := a.collectCalls(ctx, objects)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "读取呼叫状态失败", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"calls": calls, "voiceAvailable": hasVoiceModem(objects), "updatedAt": time.Now().UTC()})
}

func (a *api) callEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "实时通话状态不可用", errors.New("streaming unsupported"))
		return
	}
	options := []dbus.MatchOption{
		dbus.WithMatchInterface("org.freedesktop.DBus.Properties"),
		dbus.WithMatchMember("PropertiesChanged"),
		dbus.WithMatchPathNamespace(dbus.ObjectPath("/org/freedesktop/ModemManager1/Call")),
	}
	if err := a.conn.AddMatchSignal(options...); err != nil {
		writeError(w, http.StatusServiceUnavailable, "监听通话状态失败", err)
		return
	}
	signals := make(chan *dbus.Signal, 8)
	a.conn.Signal(signals)
	defer a.conn.RemoveSignal(signals)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(": connected\n\n"))
	flusher.Flush()
	for {
		select {
		case <-r.Context().Done():
			return
		case signal := <-signals:
			if signal != nil {
				_, _ = w.Write([]byte("event: call-state\ndata: {}\n\n"))
				flusher.Flush()
			}
		}
	}
}

func (a *api) createCall(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ModemID string `json:"modemId"`
		Number  string `json:"number"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式无效", err)
		return
	}
	input.Number = normalizeCallNumber(input.Number)
	if !callNumberPattern.MatchString(input.Number) {
		writeError(w, http.StatusBadRequest, "请输入有效的电话号码", errors.New("invalid phone number"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	objects, err := a.managedObjects(ctx)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "读取调制解调器失败", err)
		return
	}
	modemPath, err := selectVoiceModem(objects, input.ModemID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error(), err)
		return
	}
	properties := map[string]dbus.Variant{"number": dbus.MakeVariant(input.Number)}
	var callPath dbus.ObjectPath
	if err := a.conn.Object(mmService, modemPath).CallWithContext(ctx, voiceInterface+".CreateCall", 0, properties).Store(&callPath); err != nil {
		writeCallError(w, "创建呼叫失败", err)
		return
	}
	if err := a.conn.Object(mmService, callPath).CallWithContext(ctx, callInterface+".Start", 0).Err; err != nil {
		writeCallError(w, "发起呼叫失败", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": callID(callPath), "status": "dialing"})
}

func (a *api) hangupCall(w http.ResponseWriter, r *http.Request) {
	path, ok := callPathFromID(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusBadRequest, "呼叫编号无效", errors.New("invalid call id"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	if err := a.conn.Object(mmService, path).CallWithContext(ctx, callInterface+".Hangup", 0).Err; err != nil {
		writeCallError(w, "挂断失败", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) sendCallDTMF(w http.ResponseWriter, r *http.Request) {
	path, ok := callPathFromID(r.PathValue("id"))
	if !ok {
		writeError(w, http.StatusBadRequest, "呼叫编号无效", errors.New("invalid call id"))
		return
	}
	var input struct {
		Tones string `json:"tones"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil || !dtmfPattern.MatchString(input.Tones) || len(input.Tones) > 32 {
		writeError(w, http.StatusBadRequest, "按键音无效", errors.New("invalid DTMF tones"))
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := a.conn.Object(mmService, path).CallWithContext(ctx, callInterface+".SendDtmf", 0, input.Tones).Err; err != nil {
		writeCallError(w, "发送按键音失败", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) collectCalls(ctx context.Context, objects managedObjects) ([]voiceCall, error) {
	result := make([]voiceCall, 0)
	for modemPath, interfaces := range objects {
		voice := interfaces[voiceInterface]
		if voice == nil {
			continue
		}
		// The Calls property in GetManagedObjects can lag behind or omit calls
		// which terminated quickly. ListCalls is the authoritative snapshot.
		var paths []dbus.ObjectPath
		if err := a.conn.Object(mmService, modemPath).CallWithContext(ctx, voiceInterface+".ListCalls", 0).Store(&paths); err != nil {
			return nil, err
		}
		for _, path := range paths {
			props := objects[path][callInterface]
			if props == nil {
				props = make(map[string]dbus.Variant)
				if err := a.conn.Object(mmService, path).CallWithContext(ctx, "org.freedesktop.DBus.Properties.GetAll", 0, callInterface).Store(&props); err != nil {
					return nil, err
				}
			}
			result = append(result, callFromProps(path, modemPath, props))
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func callFromProps(path, modemPath dbus.ObjectPath, props map[string]dbus.Variant) voiceCall {
	direction := "unknown"
	if number(props, "Direction") == 1 {
		direction = "incoming"
	} else if number(props, "Direction") == 2 {
		direction = "outgoing"
	}
	return voiceCall{ID: callID(path), ModemID: modemID(modemPath), Number: text(props, "Number"), Direction: direction, State: callState(number(props, "State")), Reason: callReason(number(props, "StateReason"))}
}
func hasVoiceModem(objects managedObjects) bool {
	for _, interfaces := range objects {
		if interfaces[voiceInterface] != nil {
			return true
		}
	}
	return false
}
func selectVoiceModem(objects managedObjects, requested string) (dbus.ObjectPath, error) {
	paths := make([]dbus.ObjectPath, 0)
	for path, interfaces := range objects {
		if interfaces[voiceInterface] != nil && (requested == "" || modemID(path) == requested) {
			paths = append(paths, path)
		}
	}
	if len(paths) == 0 {
		return "", errors.New("未找到支持语音通话的调制解调器")
	}
	sort.Slice(paths, func(i, j int) bool { return paths[i] < paths[j] })
	return paths[0], nil
}
func normalizeCallNumber(value string) string {
	return strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(strings.TrimSpace(value))
}
func callID(path dbus.ObjectPath) string {
	return strings.TrimPrefix(string(path), "/org/freedesktop/ModemManager1/Call/")
}
func callPathFromID(id string) (dbus.ObjectPath, bool) {
	if !digitsOnly(id) {
		return "", false
	}
	path := dbus.ObjectPath("/org/freedesktop/ModemManager1/Call/" + id)
	return path, path.IsValid()
}
func callState(value uint32) string {
	values := map[uint32]string{0: "unknown", 1: "dialing", 2: "ringing-out", 3: "ringing-in", 4: "active", 5: "held", 6: "waiting", 7: "terminated"}
	return fallback(values[value], "unknown")
}
func callReason(value uint32) string {
	values := map[uint32]string{0: "unknown", 1: "outgoing-started", 2: "incoming-new", 3: "accepted", 4: "terminated", 5: "refused-or-busy", 6: "error", 7: "audio-setup-failed", 8: "transferred", 9: "deflected"}
	return fallback(values[value], "unknown")
}
func writeCallError(w http.ResponseWriter, message string, err error) {
	if strings.Contains(err.Error(), "PolicyKit authorization failed") || strings.Contains(err.Error(), "not authorized") {
		writeError(w, http.StatusForbidden, "没有电话操作权限，请为 mm-web 配置 PolicyKit 授权", err)
		return
	}
	writeError(w, http.StatusBadGateway, message, err)
}
