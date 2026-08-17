package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestModemState(t *testing.T) {
	if got := modemState(8); got != "已注册" {
		t.Fatalf("modemState(8) = %q, want 已注册", got)
	}
	if got := modemState(99); got != "未知状态" {
		t.Fatalf("modemState(99) = %q, want 未知状态", got)
	}
}

func TestValidateMessage(t *testing.T) {
	for _, test := range []struct {
		number, body string
		valid        bool
	}{
		{"10086", "查询余额", true}, {"+8613800138000", "你好", true},
		{"12", "太短", false}, {"not-a-number", "你好", false}, {"10086", "", false},
	} {
		if err := validateMessage(test.number, test.body); (err == nil) != test.valid {
			t.Errorf("validateMessage(%q, %q) error = %v, valid = %v", test.number, test.body, err, test.valid)
		}
	}
}

func TestSMSState(t *testing.T) {
	want := map[uint32]string{0: "unknown", 1: "stored", 2: "receiving", 3: "received", 4: "sending", 5: "sent", 99: "unknown"}
	for state, label := range want {
		if got := smsState(state); got != label {
			t.Errorf("smsState(%d) = %q, want %q", state, got, label)
		}
	}
}

func TestWriteMessagingErrorExplainsPolicyKitFailure(t *testing.T) {
	recorder := httptest.NewRecorder()
	writeMessagingError(recorder, "创建短信失败", errors.New("PolicyKit authorization failed: not authorized"))
	if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), "PolicyKit") {
		t.Fatalf("writeMessagingError() = (%d, %q), want a clear 403", recorder.Code, recorder.Body.String())
	}
}

func TestCollectMessages(t *testing.T) {
	modemPath := dbus.ObjectPath("/org/freedesktop/ModemManager1/Modem/0")
	smsPath := dbus.ObjectPath("/org/freedesktop/ModemManager1/SMS/7")
	objects := map[dbus.ObjectPath]map[string]map[string]dbus.Variant{
		modemPath: {messagingInterface: {"Messages": dbus.MakeVariant([]dbus.ObjectPath{smsPath})}},
		smsPath: {smsInterface: {
			"Number": dbus.MakeVariant("10086"), "Text": dbus.MakeVariant("hello"),
			"PduType": dbus.MakeVariant(uint32(1)), "State": dbus.MakeVariant(uint32(3)),
			"Timestamp": dbus.MakeVariant("2026-08-17T10:00:00+08"),
		}},
	}
	messages := messageFromProps(smsPath, modemPath, objects[smsPath][smsInterface])
	if messages.ID != "7" || messages.Direction != "received" || messages.Text != "hello" {
		t.Fatalf("messageFromProps() = %#v", messages)
	}
}

func TestStaticFilesServesAssetsAndSPAFallback(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("app shell"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app.js"), []byte("asset"), 0o600); err != nil {
		t.Fatal(err)
	}

	handler := staticFiles(dir)
	for path, want := range map[string]string{"/app.js": "asset", "/settings/modems": "app shell"} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusOK || recorder.Body.String() != want {
			t.Errorf("GET %s = (%d, %q), want (200, %q)", path, recorder.Code, recorder.Body.String(), want)
		}
	}
}

func TestAccessTechnology(t *testing.T) {
	if got := accessTechnology(1 << 14); got != "4G LTE" {
		t.Fatalf("LTE technology = %q, want 4G LTE", got)
	}
	if got := accessTechnology((1 << 15) | (1 << 14)); got != "5G NR / 4G LTE" {
		t.Fatalf("combined technology = %q, want 5G NR / 4G LTE", got)
	}
	if got := accessTechnology(0); got != "未知制式" {
		t.Fatalf("unknown technology = %q, want 未知制式", got)
	}
}
