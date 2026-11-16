//go:build windows

// Package audio talks to the Windows audio stack without cgo: WinRT for the
// Bluetooth A2DP sink (AudioPlaybackConnection) and classic COM (MMDevice /
// WASAPI sessions) for the output device and the stream level meter.
package audio

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	combase = windows.NewLazySystemDLL("combase.dll")
	ole32   = windows.NewLazySystemDLL("ole32.dll")

	procWindowsCreateString       = combase.NewProc("WindowsCreateString")
	procWindowsDeleteString       = combase.NewProc("WindowsDeleteString")
	procWindowsGetStringRawBuffer = combase.NewProc("WindowsGetStringRawBuffer")
	procRoGetActivationFactory    = combase.NewProc("RoGetActivationFactory")
	procCoCreateInstance          = ole32.NewProc("CoCreateInstance")
	procCoTaskMemFree             = ole32.NewProc("CoTaskMemFree")
	procPropVariantClear          = ole32.NewProc("PropVariantClear")
)

func mustGUID(s string) windows.GUID {
	g, err := windows.GUIDFromString("{" + s + "}")
	if err != nil {
		panic(err)
	}
	return g
}

var (
	iidAsyncInfo = mustGUID("00000036-0000-0000-C000-000000000046")
	iidClosable  = mustGUID("30D5A829-7FA4-4026-83BB-D75BAE4EA99E")
)

// HResult is a failed COM/WinRT call.
type HResult uint32

func (h HResult) Error() string { return fmt.Sprintf("0x%08X %s", uint32(h), windows.Errno(h).Error()) }

func hr(r uintptr) error {
	if int32(r) < 0 {
		return HResult(uint32(r))
	}
	return nil
}

// ptr turns an address returned by Windows (memory Go does not manage) into
// a pointer.
func ptr(addr uintptr) unsafe.Pointer { return *(*unsafe.Pointer)(unsafe.Pointer(&addr)) }

// vcall invokes method #idx of a COM object's vtable.
//
//go:uintptrescapes
func vcall(obj uintptr, idx int, args ...uintptr) uintptr {
	vtbl := *(*uintptr)(ptr(obj))
	fn := *(*uintptr)(ptr(vtbl + uintptr(idx)*unsafe.Sizeof(uintptr(0))))
	all := make([]uintptr, 0, len(args)+1)
	all = append(all, obj)
	all = append(all, args...)
	r, _, _ := syscall.SyscallN(fn, all...)
	return r
}

func release(obj uintptr) {
	if obj != 0 {
		vcall(obj, 2)
	}
}

func queryInterface(obj uintptr, iid *windows.GUID) (uintptr, error) {
	var out uintptr
	err := hr(vcall(obj, 0, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&out))))
	return out, err
}

func newHString(s string) (uintptr, error) {
	if s == "" {
		return 0, nil
	}
	u := utf16.Encode([]rune(s))
	var h uintptr
	r, _, _ := procWindowsCreateString.Call(uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)), uintptr(unsafe.Pointer(&h)))
	return h, hr(r)
}

func deleteHString(h uintptr) {
	if h != 0 {
		procWindowsDeleteString.Call(h)
	}
}

// takeHString converts and frees an HSTRING returned by a WinRT getter.
func takeHString(h uintptr) string {
	if h == 0 {
		return ""
	}
	defer deleteHString(h)
	var n uint32
	p, _, _ := procWindowsGetStringRawBuffer.Call(h, uintptr(unsafe.Pointer(&n)))
	if p == 0 || n == 0 {
		return ""
	}
	return string(utf16.Decode(unsafe.Slice((*uint16)(ptr(p)), n)))
}

// takeCoString converts and frees a CoTaskMemAlloc'ed UTF-16 string.
func takeCoString(p uintptr) string {
	if p == 0 {
		return ""
	}
	defer procCoTaskMemFree.Call(p)
	return windows.UTF16PtrToString((*uint16)(ptr(p)))
}

func activationFactory(class string, iid *windows.GUID) (uintptr, error) {
	h, err := newHString(class)
	if err != nil {
		return 0, err
	}
	defer deleteHString(h)
	var f uintptr
	r, _, _ := procRoGetActivationFactory.Call(h, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&f)))
	if err := hr(r); err != nil {
		return 0, fmt.Errorf("activation factory %s: %w", class, err)
	}
	return f, nil
}

const clsctxAll = 0x17

func coCreate(clsid, iid *windows.GUID) (uintptr, error) {
	var out uintptr
	r, _, _ := procCoCreateInstance.Call(uintptr(unsafe.Pointer(clsid)), 0, clsctxAll, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&out)))
	return out, hr(r)
}

var errTimeout = errors.New("timed out")

// waitAsync polls an IAsyncOperation / IAsyncAction until it finishes.
func waitAsync(op uintptr, timeout time.Duration) error {
	info, err := queryInterface(op, &iidAsyncInfo)
	if err != nil {
		return err
	}
	defer release(info)
	deadline := time.Now().Add(timeout)
	for {
		var status int32
		if err := hr(vcall(info, 7, uintptr(unsafe.Pointer(&status)))); err != nil {
			return err
		}
		switch status {
		case 1: // Completed
			return nil
		case 2: // Canceled
			return errors.New("canceled")
		case 3: // Error
			var code int32
			vcall(info, 8, uintptr(unsafe.Pointer(&code)))
			if e := hr(uintptr(uint32(code))); e != nil {
				return e
			}
			return errors.New("async operation failed")
		}
		if time.Now().After(deadline) {
			vcall(info, 9) // Cancel
			return errTimeout
		}
		time.Sleep(20 * time.Millisecond)
	}
}

var mtaOnce sync.Once

// Run executes f on an OS thread in the multithreaded apartment. One thread
// stays in the MTA for the life of the process so objects created here remain
// valid on any later Run call.
func Run(f func()) {
	mtaOnce.Do(func() {
		ready := make(chan struct{})
		go func() {
			runtime.LockOSThread()
			_ = windows.CoInitializeEx(0, windows.COINIT_MULTITHREADED)
			close(ready)
			select {}
		}()
		<-ready
	})
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := windows.CoInitializeEx(0, windows.COINIT_MULTITHREADED); err == nil || err == syscall.Errno(1) /* S_FALSE */ {
		defer windows.CoUninitialize()
	}
	f()
}
