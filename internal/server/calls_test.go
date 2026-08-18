package server

import (
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestNormalizeCallNumber(t *testing.T) {
	if got := normalizeCallNumber(" +86 (138) 0013-8000 "); got != "+8613800138000" {
		t.Fatalf("normalizeCallNumber = %q", got)
	}
}

func TestCallMappings(t *testing.T) {
	path := dbus.ObjectPath("/org/freedesktop/ModemManager1/Call/4")
	modem := dbus.ObjectPath("/org/freedesktop/ModemManager1/Modem/0")
	call := callFromProps(path, modem, map[string]dbus.Variant{"Number": dbus.MakeVariant("10086"), "Direction": dbus.MakeVariant(uint32(2)), "State": dbus.MakeVariant(uint32(4)), "StateReason": dbus.MakeVariant(uint32(3))})
	if call.ID != "4" || call.Number != "10086" || call.Direction != "outgoing" || call.State != "active" || call.Reason != "accepted" {
		t.Fatalf("callFromProps = %#v", call)
	}
}

func TestSelectVoiceModem(t *testing.T) {
	path := dbus.ObjectPath("/org/freedesktop/ModemManager1/Modem/2")
	objects := managedObjects{path: {voiceInterface: {"Calls": dbus.MakeVariant([]dbus.ObjectPath{})}}}
	got, err := selectVoiceModem(objects, "2")
	if err != nil || got != path {
		t.Fatalf("selectVoiceModem = %q, %v", got, err)
	}
}

func TestCallState(t *testing.T) {
	for value, want := range map[uint32]string{1: "dialing", 2: "ringing-out", 3: "ringing-in", 4: "active", 7: "terminated", 99: "unknown"} {
		if got := callState(value); got != want {
			t.Errorf("callState(%d)=%q want %q", value, got, want)
		}
	}
}
