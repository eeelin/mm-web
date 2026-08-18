package server

import (
	"strings"
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestModemDetailMappings(t *testing.T) {
	props := map[string]dbus.Variant{
		"Ports":          dbus.MakeVariant([][]interface{}{{"wwan0", uint32(2)}, {"ttyUSB2", uint32(3)}}),
		"SupportedModes": dbus.MakeVariant([][]interface{}{{uint32(14), uint32(0)}}),
		"CurrentModes":   dbus.MakeVariant([]interface{}{uint32(14), uint32(0)}),
		"UnlockRetries":  dbus.MakeVariant(map[uint32]uint32{2: 3, 4: 10}),
	}
	if got := strings.Join(modemPorts(props), ","); got != "wwan0 (net),ttyUSB2 (at)" {
		t.Fatalf("ports = %q", got)
	}
	if got := currentModes(props["CurrentModes"]); got != "允许 2G / 3G / 4G · 首选 无" {
		t.Fatalf("current modes = %q", got)
	}
	if got := supportedModes(props["SupportedModes"]); got != "允许 2G / 3G / 4G · 首选 无" {
		t.Fatalf("supported modes = %q", got)
	}
	if got := strings.Join(unlockRetries(props["UnlockRetries"]), ","); got != "SIM PIN 3 次,SIM PUK 10 次" {
		t.Fatalf("unlock retries = %q", got)
	}
	if got := ipFamilies(7); got != "IPv4 / IPv6 / IPv4v6" {
		t.Fatalf("IP families = %q", got)
	}
}
