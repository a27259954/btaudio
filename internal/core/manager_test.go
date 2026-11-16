package core

import (
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// --- fakes -------------------------------------------------------------------

type codeErr string

func (e codeErr) Error() string { return string(e) }
func (e codeErr) Code() string  { return string(e) }

type fakeConn struct{ opened, closed atomic.Bool }

func (c *fakeConn) Opened() bool { return c.opened.Load() }
func (c *fakeConn) Close()       { c.closed.Store(true); c.opened.Store(false) }

type fakeBackend struct {
	mu    sync.Mutex
	srcs  []Source
	fail  int // number of upcoming Open calls that fail
	opens int
	conns []*fakeConn
}

func (b *fakeBackend) Run(f func())       { f() }
func (b *fakeBackend) Output() string     { return "Speakers" }
func (b *fakeBackend) Meter(string) Meter { return nil }
func (b *fakeBackend) Sources() ([]Source, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.srcs, nil
}

func (b *fakeBackend) Open(string) (Conn, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.opens++
	if b.fail > 0 {
		b.fail--
		return nil, codeErr("timeout")
	}
	c := &fakeConn{}
	c.opened.Store(true)
	b.conns = append(b.conns, c)
	return c, nil
}

func (b *fakeBackend) setFail(n int) { b.mu.Lock(); b.fail = n; b.mu.Unlock() }
func (b *fakeBackend) openCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.opens
}
func (b *fakeBackend) lastConn() *fakeConn {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.conns[len(b.conns)-1]
}

type memStore struct {
	mu sync.Mutex
	m  map[string]bool
}

func (s *memStore) Auto(id string) bool { s.mu.Lock(); defer s.mu.Unlock(); return s.m[id] }
func (s *memStore) SetAuto(id string, on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[id] = on
}

var fast = Timing{
	Tick:        time.Millisecond,
	Grace:       50 * time.Millisecond,
	ReopenDelay: time.Millisecond,
	RetryGap:    time.Millisecond,
	Backoff:     []time.Duration{100 * time.Millisecond, 200 * time.Millisecond},
}

func setup(t *testing.T) (*Manager, *fakeBackend, *memStore) {
	t.Helper()
	be := &fakeBackend{srcs: []Source{{ID: "p1", Name: "Phone"}}}
	st := &memStore{m: map[string]bool{}}
	m := New(be, st, fast)
	m.refresh()
	return m, be, st
}

func status(m *Manager) Status { return m.State().Devices[0].Status }

func waitFor(t *testing.T, m *Manager, want Status) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for status(m) != want {
		if time.Now().After(deadline) {
			t.Fatalf("status = %s, want %s", status(m), want)
		}
		time.Sleep(time.Millisecond)
	}
}

// --- tests -------------------------------------------------------------------

func TestConnectTurnsOnAuto(t *testing.T) {
	m, _, st := setup(t)
	m.Connect("p1")
	waitFor(t, m, Connected)
	if !st.Auto("p1") {
		t.Fatal("Connect should enable auto-connect")
	}
}

func TestManualConnectRetries(t *testing.T) {
	m, be, _ := setup(t)
	be.setFail(1)
	m.Connect("p1")
	waitFor(t, m, Connected)
	if n := be.openCount(); n != 2 {
		t.Fatalf("opens = %d, want 2 (one failure, one retry)", n)
	}
}

func TestRepairReopensWithoutGoingIdle(t *testing.T) {
	m, be, _ := setup(t)
	m.Connect("p1")
	waitFor(t, m, Connected)
	first := be.lastConn()

	var mu sync.Mutex
	var seen []Status
	m.OnChange = func() {
		mu.Lock()
		seen = append(seen, status(m))
		mu.Unlock()
	}
	be.setFail(2) // the phone needs a moment after the old connection closes
	before := be.openCount()
	m.Repair()
	waitFor(t, m, Connected)

	if !first.closed.Load() {
		t.Error("old connection was not closed")
	}
	if n := be.openCount() - before; n != 3 {
		t.Errorf("opens during repair = %d, want 3", n)
	}
	mu.Lock()
	defer mu.Unlock()
	if slices.Contains(seen, Idle) {
		t.Errorf("repair passed through idle: %v", seen)
	}
}

func TestDropReconnectsAutomatically(t *testing.T) {
	m, be, _ := setup(t)
	m.Connect("p1")
	waitFor(t, m, Connected)
	now := time.Now()
	m.poll(now) // sees it open

	be.lastConn().opened.Store(false) // phone went away
	m.poll(now)
	if status(m) != Idle {
		t.Fatalf("drop not detected: %s", status(m))
	}
	m.poll(now) // backoff not elapsed yet
	if status(m) != Idle {
		t.Fatal("reconnected before the backoff elapsed")
	}
	m.poll(now.Add(fast.Backoff[0]))
	waitFor(t, m, Connected)
}

func TestFreshConnectionGetsGracePeriod(t *testing.T) {
	m, be, _ := setup(t)
	m.Connect("p1")
	waitFor(t, m, Connected)
	be.lastConn().opened.Store(false) // not reported open yet
	m.poll(time.Now())
	if status(m) != Connected {
		t.Fatal("a fresh connection was dropped before its grace period")
	}
	m.poll(time.Now().Add(fast.Grace * 2))
	if status(m) != Idle {
		t.Fatal("a connection that never opened was kept past the grace period")
	}
}

func TestFailuresBackOff(t *testing.T) {
	m, be, st := setup(t)
	st.SetAuto("p1", true)
	be.setFail(100)

	m.poll(time.Now())
	waitFor(t, m, Idle)
	if e := m.State().Devices[0].Error; e != "timeout" {
		t.Fatalf("error = %q, want timeout", e)
	}
	n := be.openCount()
	m.poll(time.Now()) // still inside the backoff
	if be.openCount() != n {
		t.Fatal("retried before the backoff elapsed")
	}
	m.poll(time.Now().Add(time.Second))
	waitFor(t, m, Idle)
	if be.openCount() != n+1 {
		t.Fatal("did not retry after the backoff")
	}
}

func TestDisconnectStopsAutoConnect(t *testing.T) {
	m, be, st := setup(t)
	m.Connect("p1")
	waitFor(t, m, Connected)
	c := be.lastConn()
	m.Disconnect("p1")
	if status(m) != Idle || !c.closed.Load() || st.Auto("p1") {
		t.Fatal("disconnect should close the connection and turn auto off")
	}
	n := be.openCount()
	m.poll(time.Now().Add(time.Hour))
	if be.openCount() != n {
		t.Fatal("reconnected after a manual disconnect")
	}
}

func TestUnpairedPhoneDisappears(t *testing.T) {
	m, be, _ := setup(t)
	be.mu.Lock()
	be.srcs = nil
	be.mu.Unlock()
	m.refresh()
	if len(m.State().Devices) != 0 {
		t.Fatal("unpaired idle phone should be removed")
	}
}
