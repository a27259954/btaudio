//go:build windows

package audio

import (
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows renders an AudioPlaybackConnection through a hidden capture
// endpoint ("… A2DP SNK") that svchost plays into the default output as an
// ordinary audio session. Metering that session shows whether sound arrives.

var (
	clsidMMDeviceEnumerator = mustGUID("BCDE0395-E52F-467C-8E3D-C4579291692E")
	iidIMMDeviceEnumerator  = mustGUID("A95664D2-9614-4F35-A746-DE8DB63617E6")
	iidAudioSessionManager2 = mustGUID("77AA99A0-1BD6-484F-8BC7-2C654C9A9B6F")
	iidAudioMeterInfo       = mustGUID("C02216F6-8C67-4B5B-9D00-D008E73E0064")
)

type propertyKey struct {
	fmtid windows.GUID
	pid   uint32
}

var pkeyDeviceFriendlyName = propertyKey{mustGUID("A45C254E-DF1C-4EFD-8020-67D146A850E0"), 14}

const (
	eRender     = 0
	eMultimedia = 1
)

// defaultRender returns the default playback device (caller releases).
func defaultRender() (uintptr, error) {
	enum, err := coCreate(&clsidMMDeviceEnumerator, &iidIMMDeviceEnumerator)
	if err != nil {
		return 0, err
	}
	defer release(enum)
	var dev uintptr
	err = hr(vcall(enum, 4, eRender, eMultimedia, uintptr(unsafe.Pointer(&dev)))) // GetDefaultAudioEndpoint
	return dev, err
}

func deviceName(dev uintptr) string {
	var store uintptr
	if hr(vcall(dev, 4, 0 /* STGM_READ */, uintptr(unsafe.Pointer(&store)))) != nil { // OpenPropertyStore
		return ""
	}
	defer release(store)
	var pv [24]byte // PROPVARIANT
	if hr(vcall(store, 5, uintptr(unsafe.Pointer(&pkeyDeviceFriendlyName)), uintptr(unsafe.Pointer(&pv[0])))) != nil {
		return ""
	}
	defer procPropVariantClear.Call(uintptr(unsafe.Pointer(&pv[0])))
	const vtLPWSTR = 31
	if *(*uint16)(unsafe.Pointer(&pv[0])) != vtLPWSTR {
		return ""
	}
	return windows.UTF16PtrToString(*(**uint16)(unsafe.Pointer(&pv[8])))
}

// DefaultOutput returns the friendly name of the default playback device.
func DefaultOutput() string {
	dev, err := defaultRender()
	if err != nil {
		return ""
	}
	defer release(dev)
	return deviceName(dev)
}

// Meter reads the peak level of a phone's stream on the default output.
type Meter struct{ ptr uintptr }

// FindMeter returns a meter for the phone's stream on the default output, or
// nil if Windows has no such stream right now.
func FindMeter(phone string) *Meter {
	dev, err := defaultRender()
	if err != nil {
		return nil
	}
	defer release(dev)

	var mgr uintptr
	if hr(vcall(dev, 3, uintptr(unsafe.Pointer(&iidAudioSessionManager2)), clsctxAll, 0, uintptr(unsafe.Pointer(&mgr)))) != nil { // Activate
		return nil
	}
	defer release(mgr)
	var sessions uintptr
	if hr(vcall(mgr, 5, uintptr(unsafe.Pointer(&sessions)))) != nil { // GetSessionEnumerator
		return nil
	}
	defer release(sessions)

	var n int32
	vcall(sessions, 3, uintptr(unsafe.Pointer(&n)))
	var best uintptr
	bestActive := false
	for i := int32(0); i < n; i++ {
		var ctl uintptr
		if hr(vcall(sessions, 4, uintptr(i), uintptr(unsafe.Pointer(&ctl)))) != nil {
			continue
		}
		var p uintptr
		vcall(ctl, 4, uintptr(unsafe.Pointer(&p))) // GetDisplayName
		name := takeCoString(p)
		var state int32
		vcall(ctl, 3, uintptr(unsafe.Pointer(&state))) // GetState: 0 inactive, 1 active, 2 expired
		if strings.Contains(name, "A2DP SNK") && strings.Contains(name, phone) && state != 2 && (best == 0 || (state == 1 && !bestActive)) {
			if m, err := queryInterface(ctl, &iidAudioMeterInfo); err == nil {
				release(best)
				best, bestActive = m, state == 1
			}
		}
		release(ctl)
	}
	if best == 0 {
		return nil
	}
	return &Meter{best}
}

// Peak returns the current peak level, 0..1.
func (m *Meter) Peak() float32 {
	var p float32
	if hr(vcall(m.ptr, 3, uintptr(unsafe.Pointer(&p)))) != nil { // GetPeakValue
		return 0
	}
	return p
}

// Close releases the meter.
func (m *Meter) Close() {
	release(m.ptr)
	m.ptr = 0
}
