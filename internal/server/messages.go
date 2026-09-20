package server

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/godbus/dbus/v5"
)

const (
	messagingInterface = "org.freedesktop.ModemManager1.Modem.Messaging"
	smsInterface       = "org.freedesktop.ModemManager1.Sms"
)

var phoneNumberPattern = regexp.MustCompile(`^\+?[0-9]{3,20}$`)

type message struct {
	ID             string `json:"id"`
	ModemMessageID string `json:"-"`
	ModemID        string `json:"modemId"`
	Number         string `json:"number"`
	SenderName     string `json:"senderName,omitempty"`
	Text           string `json:"text"`
	Direction      string `json:"direction"`
	State          string `json:"state"`
	Timestamp      string `json:"timestamp"`
}

func (a *api) messages(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	objects, err := a.managedObjects(ctx)
	if err != nil {
		if a.messageStore != nil {
			result, storeErr := a.messageStore.list()
			if storeErr == nil {
				writeJSON(w, http.StatusOK, map[string]any{"messages": result, "updatedAt": time.Now().UTC(), "stale": true})
				return
			}
		}
		writeError(w, http.StatusServiceUnavailable, "读取短信失败", err)
		return
	}
	result, err := a.collectMessages(ctx, objects)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "读取短信内容失败", err)
		return
	}
	if a.messageStore != nil {
		if err := a.archiveAndCleanMessages(ctx, result, false); err != nil {
			writeError(w, http.StatusInternalServerError, "保存短信到本地失败", err)
			return
		}
		result, err = a.messageStore.list()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "读取本地短信失败", err)
			return
		}
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
	input.Number, input.Text = strings.TrimSpace(input.Number), strings.TrimSpace(input.Text)
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
	properties := map[string]dbus.Variant{"number": dbus.MakeVariant(input.Number), "text": dbus.MakeVariant(input.Text)}
	var smsPath dbus.ObjectPath
	if err := a.conn.Object(mmService, modemPath).CallWithContext(ctx, messagingInterface+".Create", 0, properties).Store(&smsPath); err != nil {
		writeMessagingError(w, "创建短信失败", err)
		return
	}
	if err := a.conn.Object(mmService, smsPath).CallWithContext(ctx, smsInterface+".Send", 0).Err; err != nil {
		writeMessagingError(w, "发送短信失败", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"modemMessageId": smsID(smsPath), "status": "sent"})
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
	if a.messageStore != nil {
		a.messageSyncMu.Lock()
		defer a.messageSyncMu.Unlock()
		archived, found, err := a.messageStore.get(id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "读取本地短信失败", err)
			return
		}
		if !found {
			writeError(w, http.StatusNotFound, "短信不存在", errors.New("message not found"))
			return
		}
		current, err := a.collectMessages(ctx, objects)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "读取短信内容失败", err)
			return
		}
		for _, candidate := range current {
			if messageFingerprint(candidate) == messageFingerprint(archived) {
				if err := a.deleteModemMessage(ctx, candidate); err != nil {
					writeMessagingError(w, "删除 modem 短信失败", err)
					return
				}
			}
		}
		deleted, err := a.messageStore.delete(id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "删除本地短信失败", err)
			return
		}
		if !deleted {
			writeError(w, http.StatusNotFound, "短信不存在", errors.New("message not found"))
			return
		}
		w.WriteHeader(http.StatusNoContent)
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

func (a *api) archiveAndCleanMessages(ctx context.Context, items []message, notifications bool) error {
	if a.messageStore == nil {
		return nil
	}
	a.messageSyncMu.Lock()
	terminal := make([]message, 0, len(items))
	for _, item := range items {
		if item.State == "received" || item.State == "sent" {
			terminal = append(terminal, item)
		}
	}
	if err := a.messageStore.archive(terminal); err != nil {
		a.messageSyncMu.Unlock()
		return err
	}
	for _, item := range terminal {
		if err := a.deleteModemMessage(ctx, item); err != nil {
			log.Printf("delete archived modem SMS %s: %v", item.ID, err)
		}
	}
	if !notifications {
		a.messageSyncMu.Unlock()
		return nil
	}
	if !a.messagesReady {
		a.messagesReady = true
		err := a.messageStore.markExistingReceivedNotified()
		a.messageSyncMu.Unlock()
		return err
	}
	pending, err := a.messageStore.pendingNotifications()
	a.messageSyncMu.Unlock()
	if err != nil {
		return err
	}
	for _, item := range pending {
		report := pushReport{}
		if a.push != nil {
			report = a.push.notify(ctx, item)
		}
		if report.Subscriptions > 0 && report.Delivered == 0 {
			continue
		}
		if err := a.messageStore.markNotified(item.ID); err != nil {
			return err
		}
	}
	return nil
}

func (a *api) deleteModemMessage(ctx context.Context, item message) error {
	if a.deleteSMS != nil {
		return a.deleteSMS(ctx, item)
	}
	modemPath := dbus.ObjectPath("/org/freedesktop/ModemManager1/Modem/" + item.ModemID)
	messageID := fallback(item.ModemMessageID, item.ID)
	smsPath := dbus.ObjectPath("/org/freedesktop/ModemManager1/SMS/" + messageID)
	return a.conn.Object(mmService, modemPath).CallWithContext(ctx, messagingInterface+".Delete", 0, smsPath).Err
}

func (a *api) collectMessages(ctx context.Context, objects managedObjects) ([]message, error) {
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
	id := smsID(path)
	return message{ID: id, ModemMessageID: id, ModemID: modemID(modemPath), Number: text(props, "Number"), Text: text(props, "Text"), Direction: direction, State: smsState(number(props, "State")), Timestamp: text(props, "Timestamp")}
}

func selectMessagingModem(objects managedObjects, requested string) (dbus.ObjectPath, error) {
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

func modemForSMS(objects managedObjects, smsPath dbus.ObjectPath) (dbus.ObjectPath, error) {
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
func numericID(id string) int { value, _ := strconv.Atoi(id); return value }
func smsState(state uint32) string {
	states := map[uint32]string{0: "unknown", 1: "stored", 2: "receiving", 3: "received", 4: "sending", 5: "sent"}
	return fallback(states[state], "unknown")
}
