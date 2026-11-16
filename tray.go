package main

import (
	"strings"
	"sync"

	"github.com/a27259954/btaudio/internal/core"
	"github.com/a27259954/btaudio/internal/icon"
	"github.com/a27259954/btaudio/internal/lang"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type trayUI struct {
	mgr     *core.Manager
	tray    *application.SystemTray
	iconOn  []byte
	iconOff []byte

	mu      sync.Mutex
	lastOn  bool
	lastSig string
	lastTip string
}

func newTray(mgr *core.Manager) *trayUI {
	t := &trayUI{
		mgr:     mgr,
		tray:    app.SystemTray.New(),
		iconOn:  icon.Render(64, icon.Connected),
		iconOff: icon.Render(64, icon.Idle),
	}
	t.tray.SetIcon(t.iconOff)
	t.tray.OnClick(showWindow)
	t.tray.OnDoubleClick(showWindow)
	return t
}

func connectedNames(st UIState) []string {
	var names []string
	for _, d := range st.Devices {
		if d.Status == core.Connected {
			names = append(names, d.Name)
		}
	}
	return names
}

func (t *trayUI) buildMenu(st UIState, l lang.Tray) *application.Menu {
	menu := application.NewMenu()
	if names := connectedNames(st); len(names) > 0 {
		menu.Add("● " + l.Connected + strings.Join(names, l.Sep)).SetEnabled(false)
	} else {
		menu.Add("○ " + l.NotConnected).SetEnabled(false)
	}
	menu.AddSeparator()
	for _, d := range st.Devices {
		label := d.Name
		if d.Status == core.Connecting {
			label += l.Connecting
		}
		id, idle := d.ID, d.Status == core.Idle
		menu.AddCheckbox(label, d.Status == core.Connected).OnClick(func(*application.Context) {
			if idle {
				t.mgr.Connect(id)
			} else {
				t.mgr.Disconnect(id)
			}
		})
	}
	if len(st.Devices) == 0 {
		menu.Add(l.NoPhones).SetEnabled(false)
	}
	menu.AddSeparator()
	menu.Add(l.Repair).OnClick(func(*application.Context) { t.mgr.Repair() })
	menu.Add(l.Open).OnClick(func(*application.Context) { showWindow() })
	menu.AddSeparator()
	menu.Add(l.Quit).OnClick(func(*application.Context) { quitApp() })
	return menu
}

func (t *trayUI) update(st UIState) {
	l := lang.TrayLabels(st.Lang)
	names := connectedNames(st)
	on := len(names) > 0
	tip := "BtAudio — " + l.NotConnected
	if on {
		tip = "BtAudio — " + strings.Join(names, l.Sep)
	}
	sig := st.Lang
	for _, d := range st.Devices {
		sig += "|" + d.ID + d.Name + string(d.Status)
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	if on != t.lastOn {
		t.lastOn = on
		if on {
			t.tray.SetIcon(t.iconOn)
		} else {
			t.tray.SetIcon(t.iconOff)
		}
	}
	if tip != t.lastTip {
		t.lastTip = tip
		t.tray.SetTooltip(tip)
	}
	if sig != t.lastSig {
		t.lastSig = sig
		t.tray.SetMenu(t.buildMenu(st, l))
	}
}
