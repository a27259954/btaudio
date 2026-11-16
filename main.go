// BtAudio turns a Windows PC into a Bluetooth speaker for your phone.
package main

import (
	"embed"
	"log"
	"os"
	"slices"
	"sync/atomic"
	"time"

	"github.com/a27259954/btaudio/internal/config"
	"github.com/a27259954/btaudio/internal/core"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

// version is set at build time with -ldflags "-X main.version=v1.2.3".
var version = "dev"

func init() {
	application.RegisterEvent[UIState]("state")
	application.RegisterEvent[float32]("level")
}

var (
	app      *application.App
	window   *application.WebviewWindow
	quitting atomic.Bool
)

func quitApp() {
	quitting.Store(true)
	app.Quit()
}

func showWindow() {
	window.Show()
	window.Restore()
	window.Focus()
}

func main() {
	args := os.Args[1:]
	cfg := config.Load(configPath())
	be := newBackend(cfg)
	mgr := core.New(be, cfg, core.DefaultTiming)
	svc := &Service{mgr: mgr, cfg: cfg}

	app = application.New(application.Options{
		Name:        "BtAudio",
		Description: "Use your PC as a Bluetooth speaker for your phone",
		Services:    []application.Service{application.NewService(svc)},
		Assets:      application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: "io.github.a27259954.btaudio",
			OnSecondInstanceLaunch: func(d application.SecondInstanceData) {
				if slices.Contains(d.Args, "--quit") {
					quitApp() // `btaudio.exe --quit` closes the running copy cleanly
					return
				}
				showWindow()
			},
		},
		Windows:    application.WindowsOptions{DisableQuitOnLastWindowClosed: true},
		OnShutdown: mgr.Shutdown,
	})
	svc.app = app

	window = app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            "BtAudio",
		Width:            360,
		Height:           252,
		DisableResize:    true,
		Frameless:        true,
		Hidden:           slices.Contains(args, "--hidden"),
		BackgroundType:   application.BackgroundTypeTranslucent,
		BackgroundColour: application.NewRGBA(0, 0, 0, 0),
		Windows:          application.WindowsWindow{BackdropType: application.Mica, Theme: application.Dark},
		URL:              "/",
	})
	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if !quitting.Load() { // closing hides to the tray
			window.Hide()
			e.Cancel()
		}
	})

	tray := newTray(mgr)
	svc.onChange = func() {
		st := svc.state()
		app.Event.Emit("state", st)
		tray.update(st)
	}
	mgr.OnChange = svc.onChange

	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		if slices.Contains(args, "--quit") {
			quitApp() // nothing else was running
			return
		}
		svc.onChange()
		go mgr.Run()
		go emitLevels(mgr)
	})

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}

// emitLevels streams the phone's audio level to the UI while it is visible.
func emitLevels(mgr *core.Manager) {
	var last float32 = -1
	for range time.Tick(150 * time.Millisecond) {
		if !window.IsVisible() {
			continue
		}
		if lvl := mgr.Level(); lvl != last {
			last = lvl
			app.Event.Emit("level", lvl)
		}
	}
}
