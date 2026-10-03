//go:build windows

package trayapp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
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

func legacyStartupEnabled() bool {
	value, err := readStartupValue()
	return err == nil && strings.TrimSpace(value) != ""
}

// Task Scheduler starts the tray at logon without Explorer's Run-key delay.
// InteractiveToken keeps the tray in the user's desktop and needs no password.
const startupScript = `
$ErrorActionPreference = 'Stop'
$sid = [System.Security.Principal.WindowsIdentity]::GetCurrent().User.Value
$name = 'jlu-drcom-tray-' + $sid
$task = Get-ScheduledTask -TaskName $name -ErrorAction SilentlyContinue
switch ($env:DRCOM_STARTUP_MODE) {
  'query' { if ($task -and $task.State -ne 'Disabled') { 'enabled' }; break }
  'disable' { if ($task) { Unregister-ScheduledTask -TaskName $name -Confirm:$false }; break }
  'enable' {
    $action = New-ScheduledTaskAction -Execute $env:DRCOM_STARTUP_EXE -Argument $env:DRCOM_STARTUP_ARGS -WorkingDirectory $env:DRCOM_STARTUP_DIR
    $trigger = New-ScheduledTaskTrigger -AtLogOn -User $sid
    $principal = New-ScheduledTaskPrincipal -UserId $sid -LogonType Interactive -RunLevel Limited
    $settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -StartWhenAvailable -MultipleInstances IgnoreNew -ExecutionTimeLimit ([TimeSpan]::Zero)
    Register-ScheduledTask -TaskName $name -Action $action -Trigger $trigger -Principal $principal -Settings $settings -Force | Out-Null
    break
  }
  default { throw 'Invalid startup mode' }
}
exit 0
`

func runStartupTask(mode, configPath string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(configPath)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", startupScript)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	cmd.Env = append(os.Environ(), "DRCOM_STARTUP_MODE="+mode, "DRCOM_STARTUP_EXE="+exe,
		"DRCOM_STARTUP_ARGS=-config "+syscall.EscapeArg(abs)+" -autologin", "DRCOM_STARTUP_DIR="+filepath.Dir(exe))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("startup task %s: %w: %s", mode, err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func ConfigureStartup(mode, configPath string) error {
	if mode != "enable" && mode != "disable" {
		return fmt.Errorf("startup must be enable or disable")
	}
	if _, err := runStartupTask(mode, configPath); err != nil {
		return err
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
