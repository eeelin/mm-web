package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestModemState(t *testing.T) {
	if got := modemState(8); got != "已注册" {
		t.Fatalf("modemState(8) = %q, want 已注册", got)
	}
	if got := modemState(99); got != "未知状态" {
		t.Fatalf("modemState(99) = %q, want 未知状态", got)
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
