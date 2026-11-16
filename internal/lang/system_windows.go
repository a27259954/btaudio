package lang

import "golang.org/x/sys/windows"

// System returns the Windows display language, e.g. "zh-TW".
func System() string {
	if l, err := windows.GetUserPreferredUILanguages(windows.MUI_LANGUAGE_NAME); err == nil && len(l) > 0 {
		return l[0]
	}
	return ""
}
