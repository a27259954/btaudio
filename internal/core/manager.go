// Package core keeps phones connected as Bluetooth audio sources: it tracks
// paired phones, opens and monitors connections, reconnects after drops and
// performs the manual "repair" (a fresh reconnect). It has no Windows or UI
// dependencies; the platform is reached through Backend.
package core

import (
	"errors"
	"sort"
	"strings"
	"sync"
	"time"
)

// Status of a phone.
type Status string

const (
	Idle       Status = "idle"
	Connecting Status = "connecting"
	Connected  Status = "connected"
)

// Source is a paired phone that can stream audio.
type Source struct{ ID, Name string }

// Conn is an open audio connection.
type Conn interface {
	Opened() bool
	Close()
}

// Meter reads the level of a phone's audio stream.
type Meter interface {
	Peak() float32
	Close()
}

// Backend is the platform audio stack.
type Backend interface {
	Run(f func())                 // runs f on a thread that may use the backend
	Sources() ([]Source, error)   // paired phones
	Open(id string) (Conn, error) // blocks until streaming or failed
	Output() string               // name of the default output device
	Meter(phone string) Meter     // nil when the phone has no stream to meter
}

// Store remembers which phones connect automatically.
type Store interface {
	Auto(id string) bool
	SetAuto(id string, on bool)
}

// Device is the UI view of a phone.
type Device struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status Status `json:"status"`
	Auto   bool   `json:"auto"`
	Error  string `json:"error"` // timeout | denied | unresponsive | unsupported | failed
}

// State is everything the UI shows.
type State struct {
	Devices []Device `json:"devices"`
	Output  string   `json:"output"`
	Error   string   `json:"error"` // listing paired phones failed
}

// Timing controls the manager's pace. Tests shrink it.
type Timing struct {
	Tick        time.Duration   // poll interval
	Grace       time.Duration   // time a fresh connection may take to report Opened
	ReopenDelay time.Duration   // pause between closing and reopening on repair
	RetryGap    time.Duration   // pause between attempts within one operation
	Backoff     []time.Duration // waits between automatic reconnect attempts
}

// DefaultTiming is used by the app.
var DefaultTiming = Timing{
	Tick:        2 * time.Second,
	Grace:       8 * time.Second,
	ReopenDelay: 1500 * time.Millisecond,
	RetryGap:    2 * time.Second,
	Backoff:     []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second, 15 * time.Second, 30 * time.Second},
}

// Attempts per operation. Repair retries more because the phone may need a
// moment after the old connection closes.
const (
	autoAttempts   = 1
	manualAttempts = 2
	repairAttempts = 3
)

type device struct {
	id, name string
	status   Status
	err      string
	since    time.Time
	conn     Conn
	seenOpen bool // Opened() was true at least once
	fails    int  // consecutive failed automatic attempts
	nextTry  time.Time
	cancel   bool // Disconnect arrived while connecting
}

// Manager is safe for concurrent use.
type Manager struct {
	be    Backend
	store Store
	t     Timing

	// OnChange is called (without locks held) after any state change.
	OnChange func()

	mu      sync.Mutex
	devs    map[string]*device
	output  string
	listErr string
	meter   Meter
	level   float32
	kick    chan struct{}
}

// New creates a manager. Call Run to start it.
func New(be Backend, store Store, t Timing) *Manager {
	return &Manager{be: be, store: store, t: t, devs: map[string]*device{}, kick: make(chan struct{}, 1)}
}

func (m *Manager) notify() {
	if m.OnChange != nil {
		m.OnChange()
	}
}

// State returns a snapshot, phones sorted by name.
func (m *Manager) State() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	st := State{Output: m.output, Error: m.listErr, Devices: make([]Device, 0, len(m.devs))}
	for _, d := range m.devs {
		st.Devices = append(st.Devices, Device{ID: d.id, Name: d.name, Status: d.status, Auto: m.store.Auto(d.id), Error: d.err})
	}
	sort.Slice(st.Devices, func(i, j int) bool {
		return strings.ToLower(st.Devices[i].Name) < strings.ToLower(st.Devices[j].Name)
	})
	return st
}

// Level is the current peak (0..1) of the first connected phone's stream.
func (m *Manager) Level() float32 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.level
}

// Run blocks forever, discovering phones, watching connections and
// reconnecting automatic phones.
func (m *Manager) Run() {
	go m.be.Run(m.sample)
	m.be.Run(func() {
		m.refresh()
		m.poll(time.Now())
		tick := time.NewTicker(m.t.Tick)
		defer tick.Stop()
		for n := 1; ; n++ {
			select {
			case <-tick.C:
				if n%3 == 0 {
					m.refresh()
				}
			case <-m.kick:
				m.refresh()
			}
			m.poll(time.Now())
			m.updateMeter()
		}
	})
}

// Refresh rescans paired phones soon.
func (m *Manager) Refresh() {
	select {
	case m.kick <- struct{}{}:
	default:
	}
}

func (m *Manager) refresh() {
	srcs, err := m.be.Sources()
	out := m.be.Output()

	m.mu.Lock()
	changed := out != m.output
	m.output = out
	if err != nil {
		changed = changed || m.listErr != err.Error()
		m.listErr = err.Error()
	} else {
		changed = changed || m.listErr != ""
		m.listErr = ""
		seen := make(map[string]bool, len(srcs))
		for _, s := range srcs {
			seen[s.ID] = true
			if d := m.devs[s.ID]; d == nil {
				m.devs[s.ID] = &device{id: s.ID, name: s.Name, status: Idle}
				changed = true
			} else if d.name != s.Name {
				d.name, changed = s.Name, true
			}
		}
		for id, d := range m.devs {
			if !seen[id] && d.status == Idle { // unpaired
				delete(m.devs, id)
				changed = true
			}
		}
	}
	m.mu.Unlock()
	if changed {
		m.notify()
	}
}

// poll detects dropped connections and starts due automatic reconnects.
func (m *Manager) poll(now time.Time) {
	var dead []Conn
	var due []string
	changed := false

	m.mu.Lock()
	for _, d := range m.devs {
		switch d.status {
		case Connected:
			// A fresh connection reports Opened shortly after Open returns,
			// so Closed only counts as a drop once it was seen open.
			if d.conn.Opened() {
				d.seenOpen = true
			} else if d.seenOpen || now.Sub(d.since) > m.t.Grace {
				dead = append(dead, d.conn)
				d.conn, d.status, d.err, d.fails = nil, Idle, "", 0
				d.nextTry = now.Add(m.backoff(0))
				changed = true
			}
		case Idle:
			if m.store.Auto(d.id) && !now.Before(d.nextTry) {
				due = append(due, d.id)
			}
		}
	}
	m.mu.Unlock()

	for _, c := range dead {
		c.Close()
	}
	if changed {
		m.notify()
	}
	for _, id := range due {
		m.begin(id, autoAttempts, false)
	}
}

func (m *Manager) backoff(fails int) time.Duration {
	if len(m.t.Backoff) == 0 {
		return m.t.Tick
	}
	if fails >= len(m.t.Backoff) {
		fails = len(m.t.Backoff) - 1
	}
	return m.t.Backoff[fails]
}

func (m *Manager) updateMeter() {
	var phone string
	for _, d := range m.State().Devices {
		if d.Status == Connected {
			phone = d.Name
			break
		}
	}
	var mt Meter
	if phone != "" {
		mt = m.be.Meter(phone)
	}
	m.mu.Lock()
	old := m.meter
	m.meter = mt
	m.mu.Unlock()
	if old != nil {
		old.Close()
	}
}

func (m *Manager) sample() {
	tick := time.NewTicker(150 * time.Millisecond)
	defer tick.Stop()
	for range tick.C {
		m.mu.Lock()
		if m.meter != nil {
			m.level = m.meter.Peak()
		} else {
			m.level = 0
		}
		m.mu.Unlock()
	}
}

// Connect connects a phone and turns on its automatic connection.
func (m *Manager) Connect(id string) {
	m.store.SetAuto(id, true)
	m.mu.Lock()
	if d := m.devs[id]; d != nil {
		d.fails = 0
	}
	m.mu.Unlock()
	m.begin(id, manualAttempts, false)
}

// Disconnect closes a phone's connection and turns off its automatic connection.
func (m *Manager) Disconnect(id string) {
	m.store.SetAuto(id, false)
	m.mu.Lock()
	d := m.devs[id]
	if d == nil {
		m.mu.Unlock()
		return
	}
	var c Conn
	switch d.status {
	case Connected:
		c, d.conn, d.status = d.conn, nil, Idle
	case Connecting:
		d.cancel = true
	}
	d.err = ""
	m.mu.Unlock()
	if c != nil {
		m.be.Run(c.Close)
	}
	m.notify()
}

// SetAuto turns a phone's automatic connection on or off.
func (m *Manager) SetAuto(id string, on bool) {
	m.store.SetAuto(id, on)
	m.mu.Lock()
	if d := m.devs[id]; d != nil && on {
		d.nextTry, d.fails = time.Time{}, 0
	}
	m.mu.Unlock()
	m.notify()
}

// Repair replaces every open connection with a fresh one. Windows sometimes
// keeps a connection open but plays only silence; only a new connection
// brings the sound back. The phone stays "connecting" throughout, and the
// reopen is retried, so a repair does not leave it disconnected.
func (m *Manager) Repair() {
	var ids []string
	m.mu.Lock()
	for id, d := range m.devs {
		if d.status == Connected {
			ids = append(ids, id)
		}
	}
	m.mu.Unlock()
	for _, id := range ids {
		m.begin(id, repairAttempts, true)
	}
}

// begin starts an asynchronous connect (reopen=false, phone must be idle) or
// reconnect (reopen=true, phone must be connected).
func (m *Manager) begin(id string, attempts int, reopen bool) {
	m.mu.Lock()
	d := m.devs[id]
	if d == nil || (reopen && d.status != Connected) || (!reopen && d.status != Idle) {
		m.mu.Unlock()
		return
	}
	old := d.conn
	d.status, d.conn, d.err, d.cancel = Connecting, nil, "", false
	m.mu.Unlock()
	m.notify()

	go m.be.Run(func() {
		if old != nil {
			old.Close()
			time.Sleep(m.t.ReopenDelay)
		}
		var conn Conn
		err := errors.New("cancelled")
		for i := 0; i < attempts && !m.cancelled(id); i++ {
			if i > 0 {
				time.Sleep(m.t.RetryGap)
			}
			if conn, err = m.be.Open(id); err == nil {
				break
			}
		}
		m.finish(id, conn, err)
	})
}

func (m *Manager) cancelled(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	d := m.devs[id]
	return d == nil || d.cancel
}

func (m *Manager) finish(id string, conn Conn, err error) {
	m.mu.Lock()
	d := m.devs[id]
	switch {
	case d == nil || d.cancel:
		if d != nil {
			d.status, d.cancel = Idle, false
		}
		m.mu.Unlock()
		if conn != nil {
			conn.Close()
		}
		m.notify()
		return
	case err == nil:
		d.status, d.conn, d.since, d.seenOpen, d.fails, d.err = Connected, conn, time.Now(), false, 0, ""
	default:
		d.status, d.err = Idle, ErrorCode(err)
		d.nextTry = time.Now().Add(m.backoff(d.fails))
		d.fails++
	}
	m.mu.Unlock()
	m.notify()
}

// ErrorCode extracts a short error code (see Device.Error).
func ErrorCode(err error) string {
	var c interface{ Code() string }
	if errors.As(err, &c) {
		return c.Code()
	}
	return "failed"
}

// Shutdown closes every connection.
func (m *Manager) Shutdown() {
	var conns []Conn
	m.mu.Lock()
	for _, d := range m.devs {
		if d.conn != nil {
			conns = append(conns, d.conn)
			d.conn, d.status = nil, Idle
		}
	}
	mt := m.meter
	m.meter = nil
	m.mu.Unlock()
	m.be.Run(func() {
		if mt != nil {
			mt.Close()
		}
		for _, c := range conns {
			c.Close()
		}
	})
}
