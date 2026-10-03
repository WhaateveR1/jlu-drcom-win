//go:build windows

package trayapp

import (
	"fmt"
	"syscall"
	"unsafe"
)

// AcquireInstance must precede opening/rotating the shared log file.
func AcquireInstance() (release func(), alreadyRunning bool, err error) {
	name, _ := syscall.UTF16PtrFromString(`Local\jlu-drcom-tray`)
	h, _, e := procCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		return nil, false, fmt.Errorf("create instance mutex: %w", e)
	}
	return func() { syscall.CloseHandle(syscall.Handle(h)) }, e == syscall.ERROR_ALREADY_EXISTS, nil
}

func ShowError(message string) {
	text, _ := syscall.UTF16PtrFromString(message)
	title, _ := syscall.UTF16PtrFromString("jlu-drcom")
	procMessageBox.Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), 0x10)
}

func openPath(path string) error {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	op, _ := syscall.UTF16PtrFromString("open")
	r, _, _ := procShellExecute.Call(0, uintptr(unsafe.Pointer(op)), uintptr(unsafe.Pointer(p)), 0, 0, 1)
	if r <= 32 {
		return fmt.Errorf("could not open file/folder (Windows code %d)", r)
	}
	return nil
}

func (a *App) powerEvent(event uintptr) {
	a.mu.Lock()
	switch event {
	case 4: // PBT_APMSUSPEND
		a.suspended = true
		if a.cancel != nil {
			a.cancel()
		}
	case 18, 7: // PBT_APMRESUMEAUTOMATIC / PBT_APMRESUMESUSPEND
		a.suspended = false
	}
	restart := a.desired && !a.suspended && a.cancel == nil && !a.exiting
	a.mu.Unlock()
	if restart {
		a.startClient()
	}
}
