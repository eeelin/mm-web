package server

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

const modemInterface = "org.freedesktop.ModemManager1.Modem"

type modem struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Model          string   `json:"model"`
	State          string   `json:"state"`
	Network        string   `json:"network"`
	Tech           string   `json:"tech"`
	Signal         uint32   `json:"signal"`
	SIM            string   `json:"sim"`
	IMEI           string   `json:"imei"`
	Firmware       string   `json:"firmware"`
	Port           string   `json:"port"`
	Manufacturer   string   `json:"manufacturer"`
	DeviceID       string   `json:"deviceId"`
	Device         string   `json:"device"`
	Drivers        []string `json:"drivers"`
	Plugin         string   `json:"plugin"`
	Ports          []string `json:"ports"`
	OwnNumbers     []string `json:"ownNumbers"`
	PowerState     string   `json:"powerState"`
	Capabilities   string   `json:"capabilities"`
	SupportedModes string   `json:"supportedModes"`
	CurrentModes   string   `json:"currentModes"`
	IPFamilies     string   `json:"ipFamilies"`
	OperatorCode   string   `json:"operatorCode"`
	Registration   string   `json:"registration"`
	PacketService  string   `json:"packetService"`
	UnlockRequired string   `json:"unlockRequired"`
	UnlockRetries  []string `json:"unlockRetries"`
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
			DeviceID: text(props, "DeviceIdentifier"), Device: text(props, "Device"),
			Drivers: stringSlice(props, "Drivers"), Plugin: text(props, "Plugin"), Ports: modemPorts(props),
			OwnNumbers: stringSlice(props, "OwnNumbers"), PowerState: powerState(number(props, "PowerState")),
			Capabilities:   capabilities(number(props, "CurrentCapabilities")),
			SupportedModes: supportedModes(props["SupportedModes"]), CurrentModes: currentModes(props["CurrentModes"]),
			IPFamilies: ipFamilies(number(props, "SupportedIpFamilies")), UnlockRequired: unlockRequired(number(props, "UnlockRequired")),
			UnlockRetries: unlockRetries(props["UnlockRetries"]),
		}
		if props3gpp := interfaces["org.freedesktop.ModemManager1.Modem.Modem3gpp"]; props3gpp != nil {
			item.Network = fallback(text(props3gpp, "OperatorName"), item.Network)
			item.OperatorCode = text(props3gpp, "OperatorCode")
			item.Registration = registrationState(number(props3gpp, "RegistrationState"))
			item.PacketService = packetServiceState(number(props3gpp, "PacketServiceState"))
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

func stringSlice(props map[string]dbus.Variant, key string) []string {
	values, _ := props[key].Value().([]string)
	return values
}
func modemPorts(props map[string]dbus.Variant) []string {
	values, _ := props["Ports"].Value().([][]interface{})
	result := make([]string, 0, len(values))
	types := map[uint32]string{1: "unknown", 2: "net", 3: "at", 4: "qcdm", 5: "gps", 6: "audio"}
	for _, item := range values {
		if len(item) < 2 {
			continue
		}
		name, _ := item[0].(string)
		kind, _ := item[1].(uint32)
		result = append(result, fmt.Sprintf("%s (%s)", name, fallback(types[kind], "unknown")))
	}
	return result
}
func powerState(value uint32) string {
	return fallback(map[uint32]string{1: "关闭", 2: "低功耗", 3: "开启"}[value], "未知")
}
func capabilities(mask uint32) string {
	return maskLabels(mask, []maskLabel{{4, "GSM/UMTS"}, {8, "LTE"}, {16, "LTE Advanced"}, {32, "5G NR"}}, "未知")
}
func modeLabels(mask uint32) string {
	return maskLabels(mask, []maskLabel{{2, "2G"}, {4, "3G"}, {8, "4G"}, {16, "5G"}}, "无")
}
func currentModes(value dbus.Variant) string {
	values, _ := value.Value().([]interface{})
	if len(values) < 2 {
		return "未知"
	}
	allowed, _ := values[0].(uint32)
	preferred, _ := values[1].(uint32)
	return fmt.Sprintf("允许 %s · 首选 %s", modeLabels(allowed), modeLabels(preferred))
}
func supportedModes(value dbus.Variant) string {
	values, _ := value.Value().([][]interface{})
	result := make([]string, 0, len(values))
	for _, item := range values {
		if len(item) < 2 {
			continue
		}
		allowed, _ := item[0].(uint32)
		preferred, _ := item[1].(uint32)
		result = append(result, fmt.Sprintf("允许 %s · 首选 %s", modeLabels(allowed), modeLabels(preferred)))
	}
	return fallback(strings.Join(result, "；"), "未知")
}
func ipFamilies(mask uint32) string {
	return maskLabels(mask, []maskLabel{{1, "IPv4"}, {2, "IPv6"}, {4, "IPv4v6"}}, "未知")
}
func registrationState(value uint32) string {
	return fallback(map[uint32]string{0: "未知", 1: "空闲", 2: "本地网络", 3: "搜索中", 4: "已拒绝", 5: "漫游"}[value], "未知")
}
func packetServiceState(value uint32) string {
	return fallback(map[uint32]string{0: "未知", 1: "未附着", 2: "已附着"}[value], "未知")
}
func unlockRequired(value uint32) string {
	return fallback(map[uint32]string{1: "无", 2: "SIM PIN", 3: "SIM PIN2", 4: "SIM PUK", 5: "SIM PUK2"}[value], "未知")
}
func unlockRetries(value dbus.Variant) []string {
	values, _ := value.Value().(map[uint32]uint32)
	labels := map[uint32]string{2: "SIM PIN", 3: "SIM PIN2", 4: "SIM PUK", 5: "SIM PUK2"}
	keys := make([]int, 0, len(values))
	for key := range values {
		keys = append(keys, int(key))
	}
	sort.Ints(keys)
	result := make([]string, 0, len(keys))
	for _, key := range keys {
		result = append(result, fmt.Sprintf("%s %d 次", fallback(labels[uint32(key)], "未知"), values[uint32(key)]))
	}
	return result
}

type maskLabel struct {
	bit  uint32
	name string
}

func maskLabels(mask uint32, labels []maskLabel, empty string) string {
	result := make([]string, 0)
	for _, item := range labels {
		if mask&item.bit != 0 {
			result = append(result, item.name)
		}
	}
	return fallback(strings.Join(result, " / "), empty)
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
