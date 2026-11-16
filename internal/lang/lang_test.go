package lang

import "testing"

func TestResolve(t *testing.T) {
	cases := map[string]string{
		"":           "en",
		"en-US":      "en",
		"fr-FR":      "en",
		"zh-TW":      "zh-TW",
		"zh-Hant-HK": "zh-TW",
		"zh_hant":    "zh-TW",
		"zh-CN":      "zh-CN",
		"zh-Hans":    "zh-CN",
		"zh-SG":      "zh-CN",
		"es-419":     "es",
		"ja-JP":      "ja",
		"ko-KR":      "ko",
	}
	for in, want := range cases {
		if got := Resolve(in); got != want {
			t.Errorf("Resolve(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEveryLanguageHasTrayLabels(t *testing.T) {
	for _, l := range Supported {
		if _, ok := tray[l]; !ok {
			t.Errorf("missing tray labels for %s", l)
		}
		if !Valid(l) {
			t.Errorf("Valid(%q) = false", l)
		}
	}
	if Valid("fr") {
		t.Error("Valid(fr) = true")
	}
}
