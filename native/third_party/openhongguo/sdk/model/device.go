package model

import (
	"fmt"
	"strings"
)

// Device 是匿名身份出示的手机。注册请求体、
// 每个请求的设备参数和 User-Agent 都描述它，所以一个设备 id 始终对应一个机型。
type Device struct {
	// Brand 是 device_brand，也是制造商（"Xiaomi"）；Model 是机型代号，
	// 作为 device_type、device_model 发送，也写进 User-Agent（"23127PN0CC"）。
	Brand string `json:"brand"`
	Model string `json:"model"`
	// OSVersion 是 Android 版本（"14"），OSAPI 是其 API 级别（34），
	// Build 是 User-Agent 携带的构建 id（"UKQ1.230804.001"）。
	OSVersion string `json:"os_version"`
	OSAPI     int    `json:"os_api"`
	Build     string `json:"build"`
	// Resolution 是 "height*width"，单位是像素；DPI 是屏幕密度，
	// 单位是每英寸点数。
	Resolution string `json:"resolution"`
	DPI        int    `json:"dpi"`
	// Pad 让设备以平板身份出现（dragon_device_type=pad）。
	Pad bool `json:"pad,omitempty"`
	// Channel 是 App 构建上报的分发渠道（"xiaomi_8662_64" 来自小米应用商店）。
	Channel string `json:"channel"`
}

// DefaultDevice 是一台 Xiaomi 14，系统为 Android 14，装小米应用商店版。
var DefaultDevice = Device{
	Brand:      "Xiaomi",
	Model:      "23127PN0CC",
	OSVersion:  "14",
	OSAPI:      34,
	Build:      "UKQ1.230804.001",
	Resolution: "1200*2670",
	DPI:        460,
	Channel:    "xiaomi_8662_64",
}

// OrDefault 用 DefaultDevice 填充 d 中留空的字段。Pad 保持原值。
func (d Device) OrDefault() Device {
	def := DefaultDevice
	if d.Brand == "" {
		d.Brand = def.Brand
	}
	if d.Model == "" {
		d.Model = def.Model
	}
	if d.OSVersion == "" {
		d.OSVersion = def.OSVersion
	}
	if d.OSAPI == 0 {
		d.OSAPI = def.OSAPI
	}
	if d.Build == "" {
		d.Build = def.Build
	}
	if d.Resolution == "" {
		d.Resolution = def.Resolution
	}
	if d.DPI == 0 {
		d.DPI = def.DPI
	}
	if d.Channel == "" {
		d.Channel = def.Channel
	}
	return d
}

// Validate 拒绝会破坏 User-Agent 或查询参数的值：文本字段不得含控制字符、
// 分号或括号，数字必须为正。
func (d Device) Validate() error {
	for name, v := range map[string]string{
		"brand": d.Brand, "model": d.Model, "os_version": d.OSVersion,
		"build": d.Build, "resolution": d.Resolution, "channel": d.Channel,
	} {
		if len(v) > 64 || strings.ContainsAny(v, ";()\r\n\t") {
			return fmt.Errorf("device %s %q is not usable", name, v)
		}
	}
	if d.OSAPI < 0 || d.DPI < 0 {
		return fmt.Errorf("device os_api and dpi must be positive")
	}
	if d.Resolution != "" {
		h, w, ok := strings.Cut(d.Resolution, "*")
		if !ok || !digitsOnly(h) || !digitsOnly(w) {
			return fmt.Errorf("device resolution %q is not height*width", d.Resolution)
		}
	}
	return nil
}

func digitsOnly(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
