// Package lang resolves the UI language and holds the few strings that are
// rendered by Go (the system tray). Everything else is translated in the
// frontend (frontend/src/i18n.js).
package lang

import "strings"

// Supported lists the UI languages, in picker order.
var Supported = []string{"en", "zh-CN", "zh-TW", "es", "ja", "ko"}

// Resolve maps a BCP 47 tag (e.g. "zh-Hant-TW", "ja-JP", "es-419") to a
// supported language, falling back to English.
func Resolve(tag string) string {
	t := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(tag), "_", "-"))
	switch {
	case strings.HasPrefix(t, "zh"):
		if strings.Contains(t, "hant") || strings.HasSuffix(t, "-tw") || strings.HasSuffix(t, "-hk") || strings.HasSuffix(t, "-mo") {
			return "zh-TW"
		}
		return "zh-CN"
	case strings.HasPrefix(t, "es"):
		return "es"
	case strings.HasPrefix(t, "ja"):
		return "ja"
	case strings.HasPrefix(t, "ko"):
		return "ko"
	}
	return "en"
}

// Valid reports whether l is a supported language code.
func Valid(l string) bool {
	for _, s := range Supported {
		if s == l {
			return true
		}
	}
	return false
}

// Tray holds the system tray strings.
type Tray struct {
	NotConnected string // tooltip / status line when nothing is connected
	Connected    string // prefix before the connected phone names
	Connecting   string // suffix after a phone that is connecting
	NoPhones     string
	Repair       string
	Open         string
	Quit         string
	Sep          string // separator between phone names
}

var tray = map[string]Tray{
	"en":    {"Not connected", "Connected: ", " (connecting…)", "No paired phones", "Repair audio", "Open BtAudio", "Quit", ", "},
	"zh-CN": {"未连接", "已连接：", "（连接中…）", "没有已配对的手机", "修复声音", "打开 BtAudio", "退出", "、"},
	"zh-TW": {"未連線", "已連線：", "（連線中…）", "沒有已配對的手機", "修復聲音", "開啟 BtAudio", "結束", "、"},
	"es":    {"Sin conexión", "Conectado: ", " (conectando…)", "No hay teléfonos emparejados", "Reparar audio", "Abrir BtAudio", "Salir", ", "},
	"ja":    {"未接続", "接続中: ", "（接続しています…）", "ペアリング済みのスマホがありません", "音声を修復", "BtAudio を開く", "終了", "、"},
	"ko":    {"연결 안 됨", "연결됨: ", " (연결 중…)", "페어링된 휴대폰이 없습니다", "오디오 복구", "BtAudio 열기", "종료", ", "},
}

// TrayLabels returns the tray strings for a language.
func TrayLabels(l string) Tray {
	if t, ok := tray[l]; ok {
		return t
	}
	return tray["en"]
}
