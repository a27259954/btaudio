//go:build windows

package audio

import (
	"errors"
	"time"
	"unsafe"
)

// Windows.Media.Audio.AudioPlaybackConnection (A2DP sink, Windows 10 2004+).
var (
	iidAudioPlaybackConnectionStatics = mustGUID("E60963A2-69E6-5FFC-9E13-824A85213DAF")
	iidDeviceInformationStatics       = mustGUID("C17F100E-3A46-4A78-8013-769DC9B97390")
)

const (
	classAudioPlaybackConnection = "Windows.Media.Audio.AudioPlaybackConnection"
	classDeviceInformation       = "Windows.Devices.Enumeration.DeviceInformation"
)

// Source is a paired device that can stream audio to this PC.
type Source struct {
	ID   string
	Name string
}

// Sources lists paired devices that can act as an A2DP audio source.
func Sources() ([]Source, error) {
	apc, err := activationFactory(classAudioPlaybackConnection, &iidAudioPlaybackConnectionStatics)
	if err != nil {
		return nil, err
	}
	defer release(apc)

	var sel uintptr
	if err := hr(vcall(apc, 6, uintptr(unsafe.Pointer(&sel)))); err != nil { // GetDeviceSelector
		return nil, err
	}
	defer deleteHString(sel)

	dis, err := activationFactory(classDeviceInformation, &iidDeviceInformationStatics)
	if err != nil {
		return nil, err
	}
	defer release(dis)

	var op uintptr
	if err := hr(vcall(dis, 10, sel, uintptr(unsafe.Pointer(&op)))); err != nil { // FindAllAsync(aqs)
		return nil, err
	}
	defer release(op)
	if err := waitAsync(op, 10*time.Second); err != nil {
		return nil, err
	}

	var coll uintptr                                                         // IVectorView<DeviceInformation>
	if err := hr(vcall(op, 8, uintptr(unsafe.Pointer(&coll)))); err != nil { // GetResults
		return nil, err
	}
	defer release(coll)

	var n uint32
	vcall(coll, 7, uintptr(unsafe.Pointer(&n))) // get_Size
	out := make([]Source, 0, n)
	for i := uint32(0); i < n; i++ {
		var dev uintptr
		if hr(vcall(coll, 6, uintptr(i), uintptr(unsafe.Pointer(&dev)))) != nil { // GetAt
			continue
		}
		var id, name uintptr
		vcall(dev, 6, uintptr(unsafe.Pointer(&id)))   // get_Id
		vcall(dev, 7, uintptr(unsafe.Pointer(&name))) // get_Name
		out = append(out, Source{ID: takeHString(id), Name: takeHString(name)})
		release(dev)
	}
	return out, nil
}

// OpenError explains why a connection could not be opened.
type OpenError struct {
	code string // timeout | denied | unresponsive | unsupported | failed
	err  error
}

func (e *OpenError) Error() string {
	if e.err != nil {
		return e.code + ": " + e.err.Error()
	}
	return e.code
}
func (e *OpenError) Code() string  { return e.code }
func (e *OpenError) Unwrap() error { return e.err }

// Conn is an open AudioPlaybackConnection. Windows plays the phone's audio on
// the default output for as long as it stays open.
type Conn struct{ ptr uintptr }

// Open creates a connection to the phone and asks it to start streaming.
func Open(id string, timeout time.Duration) (*Conn, error) {
	apc, err := activationFactory(classAudioPlaybackConnection, &iidAudioPlaybackConnectionStatics)
	if err != nil {
		return nil, &OpenError{"failed", err}
	}
	defer release(apc)
	hid, err := newHString(id)
	if err != nil {
		return nil, &OpenError{"failed", err}
	}
	defer deleteHString(hid)

	c := &Conn{}
	if err := hr(vcall(apc, 7, hid, uintptr(unsafe.Pointer(&c.ptr)))); err != nil { // TryCreateFromId
		return nil, &OpenError{"failed", err}
	}
	if c.ptr == 0 {
		return nil, &OpenError{code: "unsupported"}
	}
	if err := c.open(timeout); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

func (c *Conn) open(timeout time.Duration) error {
	if err := hr(vcall(c.ptr, 6)); err != nil { // Start
		return &OpenError{"failed", err}
	}
	var op uintptr
	if err := hr(vcall(c.ptr, 11, uintptr(unsafe.Pointer(&op)))); err != nil { // OpenAsync
		return &OpenError{"failed", err}
	}
	defer release(op)
	if err := waitAsync(op, timeout); err != nil {
		if errors.Is(err, errTimeout) {
			return &OpenError{code: "timeout"}
		}
		return &OpenError{"failed", err}
	}
	var res uintptr
	if err := hr(vcall(op, 8, uintptr(unsafe.Pointer(&res)))); err != nil { // GetResults
		return &OpenError{"failed", err}
	}
	defer release(res)
	var status, ext int32
	vcall(res, 6, uintptr(unsafe.Pointer(&status))) // get_Status
	vcall(res, 7, uintptr(unsafe.Pointer(&ext)))    // get_ExtendedError
	switch status {
	case 0: // Success
		return nil
	case 1: // RequestTimedOut
		return &OpenError{code: "timeout"}
	case 2: // DeniedBySystem
		return &OpenError{code: "denied"}
	}
	err := hr(uintptr(uint32(ext)))
	if h, ok := err.(HResult); ok && h == 0x8007001F { // ERROR_GEN_FAILURE: phone did not answer
		return &OpenError{"unresponsive", err}
	}
	return &OpenError{"failed", err}
}

// Opened reports whether the connection is currently streaming.
func (c *Conn) Opened() bool {
	var state int32
	if hr(vcall(c.ptr, 9, uintptr(unsafe.Pointer(&state)))) != nil { // get_State
		return false
	}
	return state == 1
}

// Close tears the connection down.
func (c *Conn) Close() {
	if c.ptr == 0 {
		return
	}
	if cl, err := queryInterface(c.ptr, &iidClosable); err == nil {
		vcall(cl, 6) // IClosable.Close
		release(cl)
	}
	release(c.ptr)
	c.ptr = 0
}
