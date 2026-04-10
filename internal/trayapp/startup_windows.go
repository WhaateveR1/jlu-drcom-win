//go:build windows

package trayapp

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

const (
	hkeyCurrentUser uintptr = 0x80000001

	keyQueryValue = 0x0001
	keySetValue   = 0x0002

	regOptionNonVolatile = 0
	regSz                = 1

	errorFileNotFound syscall.Errno = 2
)

var (
	advapi32             = syscall.NewLazyDLL("advapi32.dll")
	procRegCreateKeyExW  = advapi32.NewProc("RegCreateKeyExW")
	procRegOpenKeyExW    = advapi32.NewProc("RegOpenKeyExW")
	procRegSetValueExW   = advapi32.NewProc("RegSetValueExW")
	procRegQueryValueExW = advapi32.NewProc("RegQueryValueExW")
	procRegDeleteValueW  = advapi32.NewProc("RegDeleteValueW")
	procRegCloseKey      = advapi32.NewProc("RegCloseKey")
)

const startupRunKey = `Software\Microsoft\Windows\CurrentVersion\Run`
const startupValueName = "jlu-drcom-tray"

func startupEnabled() bool {
	value, err := readStartupValue()
	return err == nil && strings.TrimSpace(value) != ""
}

func setStartupEnabled(enabled bool, command string) error {
	if enabled {
		return writeStartupValue(command)
	}
	return deleteStartupValue()
}

func readStartupValue() (string, error) {
	key, err := openKey(startupRunKey, keyQueryValue)
	if err != nil {
		return "", err
	}
	defer closeKey(key)

	name, err := syscall.UTF16PtrFromString(startupValueName)
	if err != nil {
		return "", err
	}
	var typ uint32
	var size uint32
	r1, _, _ := procRegQueryValueExW.Call(
		key,
		uintptr(unsafe.Pointer(name)),
		0,
		uintptr(unsafe.Pointer(&typ)),
		0,
		uintptr(unsafe.Pointer(&size)),
	)
	if syscall.Errno(r1) == errorFileNotFound {
		return "", errorFileNotFound
	}
	if r1 != 0 {
		return "", syscall.Errno(r1)
	}
	if typ != regSz || size == 0 {
		return "", fmt.Errorf("unexpected startup registry value type")
	}

	buf := make([]uint16, (size+1)/2)
	r1, _, _ = procRegQueryValueExW.Call(
		key,
		uintptr(unsafe.Pointer(name)),
		0,
		uintptr(unsafe.Pointer(&typ)),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if r1 != 0 {
		return "", syscall.Errno(r1)
	}
	return syscall.UTF16ToString(buf), nil
}

func writeStartupValue(command string) error {
	key, err := createKey(startupRunKey)
	if err != nil {
		return err
	}
	defer closeKey(key)

	name, err := syscall.UTF16PtrFromString(startupValueName)
	if err != nil {
		return err
	}
	value, err := syscall.UTF16FromString(command)
	if err != nil {
		return err
	}
	size := uint32(len(value) * 2)
	r1, _, _ := procRegSetValueExW.Call(
		key,
		uintptr(unsafe.Pointer(name)),
		0,
		regSz,
		uintptr(unsafe.Pointer(&value[0])),
		uintptr(size),
	)
	if r1 != 0 {
		return syscall.Errno(r1)
	}
	return nil
}

func deleteStartupValue() error {
	key, err := openKey(startupRunKey, keySetValue)
	if err != nil {
		if err == errorFileNotFound {
			return nil
		}
		return err
	}
	defer closeKey(key)

	name, err := syscall.UTF16PtrFromString(startupValueName)
	if err != nil {
		return err
	}
	r1, _, _ := procRegDeleteValueW.Call(key, uintptr(unsafe.Pointer(name)))
	if r1 != 0 && syscall.Errno(r1) != errorFileNotFound {
		return syscall.Errno(r1)
	}
	return nil
}

func openKey(path string, access uint32) (uintptr, error) {
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var key uintptr
	r1, _, _ := procRegOpenKeyExW.Call(
		hkeyCurrentUser,
		uintptr(unsafe.Pointer(pathPtr)),
		0,
		uintptr(access),
		uintptr(unsafe.Pointer(&key)),
	)
	if r1 != 0 {
		return 0, syscall.Errno(r1)
	}
	return key, nil
}

func createKey(path string) (uintptr, error) {
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var key uintptr
	r1, _, _ := procRegCreateKeyExW.Call(
		hkeyCurrentUser,
		uintptr(unsafe.Pointer(pathPtr)),
		0,
		0,
		regOptionNonVolatile,
		keySetValue,
		0,
		uintptr(unsafe.Pointer(&key)),
		0,
	)
	if r1 != 0 {
		return 0, syscall.Errno(r1)
	}
	return key, nil
}

func closeKey(key uintptr) {
	procRegCloseKey.Call(key)
}
