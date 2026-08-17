package server

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

const modemInterface = "org.freedesktop.ModemManager1.Modem"

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

func (a *api) modems(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	objects, err := a.managedObjects(ctx)
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "读取 ModemManager 失败", err)
		return
	}

	result := make([]modem, 0)
	for path, interfaces := range objects {
		props, ok := interfaces[modemInterface]
		if !ok {
			continue
		}
		item := modem{
			ID: modemID(path), Name: joinName(text(props, "Manufacturer"), text(props, "Model")),
			Model: fallback(text(props, "Model"), "未知型号"), State: modemState(number(props, "State")),
			Network: "未注册网络", Tech: accessTechnology(number(props, "AccessTechnologies")),
			Signal: signalQuality(props["SignalQuality"]), SIM: "未插入 SIM 卡",
			IMEI: text(props, "EquipmentIdentifier"), Firmware: text(props, "Revision"),
			Port: text(props, "PrimaryPort"), Manufacturer: text(props, "Manufacturer"),
		}
		if props3gpp := interfaces["org.freedesktop.ModemManager1.Modem.Modem3gpp"]; props3gpp != nil {
			item.Network = fallback(text(props3gpp, "OperatorName"), item.Network)
		}
		if simPath, ok := props["Sim"].Value().(dbus.ObjectPath); ok && simPath.IsValid() && simPath != "/" {
			item.SIM = "SIM · 就绪"
			if simProps := objects[simPath]["org.freedesktop.ModemManager1.Sim"]; simProps != nil {
				if operator := text(simProps, "OperatorName"); operator != "" && item.Network == "未注册网络" {
					item.Network = operator
				}
			}
		}
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	writeJSON(w, http.StatusOK, map[string]any{"modems": result, "updatedAt": time.Now().UTC()})
}

func signalQuality(value dbus.Variant) uint32 {
	values, ok := value.Value().([]interface{})
	if ok && len(values) > 0 {
		if result, ok := values[0].(uint32); ok {
			return result
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
	for _, item := range types {
		if mask&item.bit != 0 {
			names = append(names, item.name)
		}
	}
	if len(names) == 0 {
		return "未知制式"
	}
	return strings.Join(names, " / ")
}

func joinName(manufacturer, model string) string {
	return fallback(strings.TrimSpace(manufacturer+" "+model), "调制解调器")
}
func modemID(path dbus.ObjectPath) string {
	return strings.TrimPrefix(string(path), "/org/freedesktop/ModemManager1/Modem/")
}
