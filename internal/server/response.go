package server

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
)

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
