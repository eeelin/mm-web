package server

import (
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestSignalQualityRecent(t *testing.T) {
	if !signalQualityRecent(dbus.MakeVariant([]interface{}{uint32(60), true})) {
		t.Fatal("fresh signal quality was considered stale")
	}
	if signalQualityRecent(dbus.MakeVariant([]interface{}{uint32(60), false})) {
		t.Fatal("cached signal quality was considered fresh")
	}
}

func TestModemHealthAlertsAfterThreeFailuresAndDeduplicates(t *testing.T) {
	monitor := newModemHealthMonitor(nil)
	healthy := modemHealthState{DBusAvailable: true, ModemsOnline: true, ATAvailable: true}
	failed := modemHealthState{DBusAvailable: true, ModemsOnline: true, ATAvailable: false}
	monitor.observe(healthy)
	monitor.observe(failed)
	monitor.observe(failed)
	if monitor.notifiedDown["at"] {
		t.Fatal("alert fired before threshold")
	}
	monitor.observe(failed)
	if !monitor.notifiedDown["at"] {
		t.Fatal("alert did not fire at threshold")
	}
	monitor.observe(failed)
	if monitor.failures["at"] != 4 || !monitor.notifiedDown["at"] {
		t.Fatal("ongoing incident was not deduplicated")
	}
	monitor.observe(healthy)
	if monitor.notifiedDown["at"] || monitor.failures["at"] != 0 {
		t.Fatal("recovery did not reset incident")
	}
}

func TestModemHealthPushContent(t *testing.T) {
	content := modemHealthPushContent("at")
	if content["tag"] != "modem-health-at" || content["title"] == "" || content["body"] == "" {
		t.Fatalf("unexpected health push: %#v", content)
	}
}

func TestModemHealthDoesNotCascadeDBusFailure(t *testing.T) {
	monitor := newModemHealthMonitor(nil)
	monitor.observe(modemHealthState{DBusAvailable: true, ModemsOnline: true, ATAvailable: true})
	failed := modemHealthState{}
	monitor.observe(failed)
	monitor.observe(failed)
	monitor.observe(failed)
	if !monitor.notifiedDown["dbus"] {
		t.Fatal("D-Bus outage did not alert")
	}
	if monitor.notifiedDown["modem"] || monitor.notifiedDown["at"] {
		t.Fatal("D-Bus outage generated cascading modem or AT alerts")
	}
}
