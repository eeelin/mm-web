package server

import (
	"context"
	"strings"

	"github.com/godbus/dbus/v5"
)

type managedObjects map[dbus.ObjectPath]map[string]map[string]dbus.Variant

func (a *api) managedObjects(ctx context.Context) (managedObjects, error) {
	var objects managedObjects
	err := a.conn.Object(mmService, "/org/freedesktop/ModemManager1").CallWithContext(ctx, "org.freedesktop.DBus.ObjectManager.GetManagedObjects", 0).Store(&objects)
	return objects, err
}

func text(props map[string]dbus.Variant, key string) string {
	value, ok := props[key]
	if !ok {
		return ""
	}
	result, _ := value.Value().(string)
	return result
}

func number(props map[string]dbus.Variant, key string) uint32 {
	value, ok := props[key]
	if !ok {
		return 0
	}
	switch result := value.Value().(type) {
	case uint32:
		return result
	case int32:
		return uint32(result)
	default:
		return 0
	}
}

func fallback(value, defaultValue string) string {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}
