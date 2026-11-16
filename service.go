package main

import (
	"github.com/a27259954/btaudio/internal/config"
	"github.com/a27259954/btaudio/internal/core"
	"github.com/a27259954/btaudio/internal/lang"

	"github.com/wailsapp/wails/v3/pkg/application"
	"golang.org/x/sys/windows"
)

// UIState is what the frontend renders.
type UIState struct {
	Devices     []core.Device `json:"devices"`
	Output      string        `json:"output"`
	Error       string        `json:"error"`
	Autostart   bool          `json:"autostart"`
	Lang        string        `json:"lang"`        // effective UI language
	LangSetting string        `json:"langSetting"` // "" follows Windows
	Version     string        `json:"version"`
}

// Service is the API exposed to the frontend.
type Service struct {
	mgr      *core.Manager
	cfg      *config.Config
	app      *application.App
	onChange func()
}

func (s *Service) state() UIState {
	st := s.mgr.State()
	on, _ := s.app.Autostart.IsEnabled()
	return UIState{
		Devices: st.Devices, Output: st.Output, Error: st.Error,
		Autostart: on, Lang: s.language(), LangSetting: s.cfg.Language(), Version: version,
	}
}

// language returns the effective UI language.
func (s *Service) language() string {
	if l := s.cfg.Language(); lang.Valid(l) {
		return l
	}
	if l := langOverride(); l != "" {
		return lang.Resolve(l)
	}
	return lang.Resolve(lang.System())
}

func (s *Service) GetState() UIState          { return s.state() }
func (s *Service) Connect(id string)          { s.mgr.Connect(id) }
func (s *Service) Disconnect(id string)       { s.mgr.Disconnect(id) }
func (s *Service) SetAuto(id string, on bool) { s.mgr.SetAuto(id, on) }
func (s *Service) Repair()                    { s.mgr.Repair() }
func (s *Service) Quit()                      { quitApp() }

// SetAutostart starts BtAudio (in the tray) when the user signs in.
func (s *Service) SetAutostart(on bool) error {
	var err error
	if on {
		err = s.app.Autostart.EnableWithOptions(application.AutostartOptions{Arguments: []string{"--hidden"}})
	} else {
		err = s.app.Autostart.Disable()
	}
	s.onChange()
	return err
}

// SetLanguage sets the UI language; "" follows Windows.
func (s *Service) SetLanguage(l string) {
	if l != "" && !lang.Valid(l) {
		return
	}
	s.cfg.SetLanguage(l)
	s.onChange()
}

// OpenBluetoothSettings opens Windows Settings → Bluetooth & devices.
func (s *Service) OpenBluetoothSettings() {
	verb, _ := windows.UTF16PtrFromString("open")
	uri, _ := windows.UTF16PtrFromString("ms-settings:bluetooth")
	_ = windows.ShellExecute(0, verb, uri, nil, nil, windows.SW_SHOWNORMAL)
}
