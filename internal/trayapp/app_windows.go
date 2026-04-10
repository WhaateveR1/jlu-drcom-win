//go:build windows

package trayapp

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"jlu-drcom-win/internal/config"
	"jlu-drcom-win/internal/runner"
	"jlu-drcom-win/internal/transport"
)

const (
	windowClassName = "jluDrcomTrayWindow"
	trayIconID      = 1

	wmClose       = 0x0010
	wmDestroy     = 0x0002
	wmCommand     = 0x0111
	wmUser        = 0x0400
	wmTray        = wmUser + 1
	wmMouseMove   = 0x0200
	wmLButtonUp   = 0x0202
	wmLButtonDbl  = 0x0203
	wmRButtonUp   = 0x0205
	wmRButtonDbl  = 0x0206
	wmContextMenu = 0x007b
	ninSelect     = wmUser
	ninKeySelect  = wmUser + 1

	nimAdd        = 0x00000000
	nimModify     = 0x00000001
	nimDelete     = 0x00000002
	nimSetVersion = 0x00000004

	nifMessage = 0x00000001
	nifIcon    = 0x00000002
	nifTip     = 0x00000004

	notifyIconVersion4 = 4
	idiApplication     = 32512

	mfString    = 0x00000000
	mfGrayed    = 0x00000001
	mfChecked   = 0x00000008
	mfSeparator = 0x00000800

	tpmRightButton = 0x00000002
	tpmRetCmd      = 0x00000100

	menuStatus  = 100
	menuLogin   = 101
	menuLogout  = 102
	menuStartup = 103
	menuExit    = 104
)

var (
	user32              = syscall.NewLazyDLL("user32.dll")
	kernel32            = syscall.NewLazyDLL("kernel32.dll")
	shell32             = syscall.NewLazyDLL("shell32.dll")
	procRegisterClassEx = user32.NewProc("RegisterClassExW")
	procCreateWindowEx  = user32.NewProc("CreateWindowExW")
	procDefWindowProc   = user32.NewProc("DefWindowProcW")
	procDestroyWindow   = user32.NewProc("DestroyWindow")
	procPostQuitMessage = user32.NewProc("PostQuitMessage")
	procGetMessage      = user32.NewProc("GetMessageW")
	procTranslateMsg    = user32.NewProc("TranslateMessage")
	procDispatchMsg     = user32.NewProc("DispatchMessageW")
	procPostMessage     = user32.NewProc("PostMessageW")
	procLoadIcon        = user32.NewProc("LoadIconW")
	procCreatePopupMenu = user32.NewProc("CreatePopupMenu")
	procAppendMenu      = user32.NewProc("AppendMenuW")
	procDestroyMenu     = user32.NewProc("DestroyMenu")
	procTrackPopupMenu  = user32.NewProc("TrackPopupMenu")
	procGetCursorPos    = user32.NewProc("GetCursorPos")
	procSetForeground   = user32.NewProc("SetForegroundWindow")
	procGetModuleHandle = kernel32.NewProc("GetModuleHandleW")
	procShellNotifyIcon = shell32.NewProc("Shell_NotifyIconW")

	activeMu  sync.Mutex
	activeApp *App
)

type App struct {
	cfg        config.Config
	configPath string
	rng        io.Reader
	logger     *slog.Logger

	hwnd uintptr
	nid  notifyIconData

	mu      sync.Mutex
	status  string
	cancel  context.CancelFunc
	done    chan error
	runner  *runner.Runner
	exiting bool
}

func New(cfg config.Config, configPath string, rng io.Reader, logger *slog.Logger) *App {
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &App{
		cfg:        cfg,
		configPath: configPath,
		rng:        rng,
		logger:     logger,
		status:     string(runner.StateStopped),
	}
}

func (a *App) Run() error {
	setActiveApp(a)
	defer setActiveApp(nil)

	if err := a.createWindow(); err != nil {
		return err
	}
	if err := a.addTrayIcon(); err != nil {
		procDestroyWindow.Call(a.hwnd)
		return err
	}
	a.setStatus("Stopped")
	return messageLoop()
}

func (a *App) createWindow() error {
	hInstance, _, _ := procGetModuleHandle.Call(0)
	className, err := syscall.UTF16PtrFromString(windowClassName)
	if err != nil {
		return err
	}
	wndProc := syscall.NewCallback(windowProc)
	wc := wndClassEx{
		cbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
		lpfnWndProc:   wndProc,
		hInstance:     hInstance,
		lpszClassName: className,
	}
	r1, _, err := procRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc)))
	if r1 == 0 {
		return fmt.Errorf("RegisterClassExW: %w", err)
	}

	windowName, err := syscall.UTF16PtrFromString("jlu-drcom tray")
	if err != nil {
		return err
	}
	hwnd, _, err := procCreateWindowEx.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windowName)),
		0,
		0, 0, 0, 0,
		0, 0, hInstance, 0,
	)
	if hwnd == 0 {
		return fmt.Errorf("CreateWindowExW: %w", err)
	}
	a.hwnd = hwnd
	return nil
}

func (a *App) addTrayIcon() error {
	icon, _, _ := procLoadIcon.Call(0, idiApplication)
	a.nid = notifyIconData{
		cbSize:           uint32(unsafe.Sizeof(notifyIconData{})),
		hWnd:             a.hwnd,
		uID:              trayIconID,
		uFlags:           nifMessage | nifIcon | nifTip,
		uCallbackMessage: wmTray,
		hIcon:            icon,
	}
	a.setTipLocked("jlu-drcom: Stopped")
	r1, _, err := procShellNotifyIcon.Call(nimAdd, uintptr(unsafe.Pointer(&a.nid)))
	if r1 == 0 {
		return fmt.Errorf("Shell_NotifyIconW(NIM_ADD): %w", err)
	}
	a.nid.uTimeoutOrVersion = notifyIconVersion4
	procShellNotifyIcon.Call(nimSetVersion, uintptr(unsafe.Pointer(&a.nid)))
	return nil
}

func (a *App) removeTrayIcon() {
	if a.hwnd == 0 {
		return
	}
	a.nid.uFlags = 0
	procShellNotifyIcon.Call(nimDelete, uintptr(unsafe.Pointer(&a.nid)))
}

func (a *App) setStatus(status string) {
	a.mu.Lock()
	a.status = status
	a.setTipLocked("jlu-drcom: " + status)
	a.mu.Unlock()
	a.modifyTip()
}

func (a *App) setTipLocked(tip string) {
	a.nid.szTip = [128]uint16{}
	writeUTF16(a.nid.szTip[:], tip)
}

func (a *App) modifyTip() {
	a.mu.Lock()
	a.nid.uFlags = nifTip
	nid := a.nid
	a.mu.Unlock()
	procShellNotifyIcon.Call(nimModify, uintptr(unsafe.Pointer(&nid)))
}

func (a *App) showMenu() {
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer procDestroyMenu.Call(menu)

	status, running := a.snapshot()
	appendMenu(menu, mfString|mfGrayed, menuStatus, "Status: "+status)
	appendSeparator(menu)
	loginFlags := uint32(mfString)
	logoutFlags := uint32(mfString)
	if running {
		loginFlags |= mfGrayed
	} else {
		logoutFlags |= mfGrayed
	}
	appendMenu(menu, loginFlags, menuLogin, "Login")
	appendMenu(menu, logoutFlags, menuLogout, "Logout")
	appendSeparator(menu)
	startupFlags := uint32(mfString)
	if startupEnabled() {
		startupFlags |= mfChecked
	}
	appendMenu(menu, startupFlags, menuStartup, "Start with Windows")
	appendSeparator(menu)
	appendMenu(menu, mfString, menuExit, "Exit")

	var p point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&p)))
	procSetForeground.Call(a.hwnd)
	cmd, _, _ := procTrackPopupMenu.Call(
		menu,
		tpmRightButton|tpmRetCmd,
		uintptr(p.x),
		uintptr(p.y),
		0,
		a.hwnd,
		0,
	)
	a.handleMenu(uint16(cmd))
}

func (a *App) handleMenu(cmd uint16) {
	switch cmd {
	case menuLogin:
		a.startClient()
	case menuLogout:
		a.stopClient()
	case menuStartup:
		a.toggleStartup()
	case menuExit:
		a.requestExit()
	}
}

func (a *App) startClient() {
	a.mu.Lock()
	if a.cancel != nil {
		a.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	a.cancel = cancel
	a.done = done
	a.exiting = false
	a.status = "Starting"
	a.setTipLocked("jlu-drcom: Starting")
	a.mu.Unlock()
	a.modifyTip()

	factory := func() (runner.Exchanger, error) {
		udpTransport, err := transport.NewTransport(a.cfg.BindUDPAddr(), a.cfg.ServerUDPAddr(), a.cfg.ReceiveTimeout)
		if err != nil {
			return nil, err
		}
		a.logger.Info("udp socket bound", "bind", a.cfg.BindAddrString(), "server", a.cfg.ServerAddrString())
		return udpTransport, nil
	}
	r := runner.NewWithTransportFactory(a.cfg, factory, a.rng, a.logger)
	a.mu.Lock()
	a.runner = r
	a.mu.Unlock()

	go a.pollRunner(ctx, r, done)
	go func() {
		err := r.Run(ctx)
		r.Close()
		done <- err
		a.runnerDone(err)
	}()
}

func (a *App) stopClient() {
	a.mu.Lock()
	cancel := a.cancel
	a.mu.Unlock()
	if cancel == nil {
		return
	}
	a.setStatus("LoggingOut")
	cancel()
}

func (a *App) requestExit() {
	a.mu.Lock()
	if a.exiting {
		a.mu.Unlock()
		return
	}
	a.exiting = true
	cancel := a.cancel
	running := cancel != nil
	a.mu.Unlock()

	if running {
		a.setStatus("LoggingOut")
		cancel()
		return
	}
	postMessage(a.hwnd, wmClose, 0, 0)
}

func (a *App) runnerDone(err error) {
	a.mu.Lock()
	exiting := a.exiting
	a.cancel = nil
	a.done = nil
	a.runner = nil
	if err != nil && !strings.Contains(err.Error(), "context canceled") {
		a.status = "Failed"
		a.logger.Error("runner stopped with error", "error", err)
	} else {
		a.status = "Stopped"
	}
	a.setTipLocked("jlu-drcom: " + a.status)
	a.mu.Unlock()
	a.modifyTip()

	if exiting {
		postMessage(a.hwnd, wmClose, 0, 0)
	}
}

func (a *App) pollRunner(ctx context.Context, r *runner.Runner, done <-chan error) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.setStatus(string(r.State()))
		}
	}
}

func (a *App) toggleStartup() {
	command, err := startupCommand(a.configPath)
	if err != nil {
		a.logger.Error("build startup command failed", "error", err)
		return
	}
	enable := !startupEnabled()
	if err := setStartupEnabled(enable, command); err != nil {
		a.logger.Error("set startup failed", "error", err)
		return
	}
	if enable {
		a.logger.Info("startup enabled")
	} else {
		a.logger.Info("startup disabled")
	}
}

func (a *App) snapshot() (status string, running bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.status, a.cancel != nil
}

func startupCommand(configPath string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return quoteWindowsArg(exe) + " -config " + quoteWindowsArg(configPath), nil
}

func quoteWindowsArg(arg string) string {
	return `"` + strings.ReplaceAll(arg, `"`, `\"`) + `"`
}

func appendMenu(menu uintptr, flags uint32, id uint16, text string) {
	ptr, _ := syscall.UTF16PtrFromString(text)
	procAppendMenu.Call(menu, uintptr(flags), uintptr(id), uintptr(unsafe.Pointer(ptr)))
}

func appendSeparator(menu uintptr) {
	procAppendMenu.Call(menu, mfSeparator, 0, 0)
}

func writeUTF16(dst []uint16, s string) {
	src := syscall.StringToUTF16(s)
	if len(src) > len(dst) {
		src = src[:len(dst)]
		src[len(src)-1] = 0
	}
	copy(dst, src)
}

func messageLoop() error {
	var m msg
	for {
		r1, _, err := procGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r1) == -1 {
			return fmt.Errorf("GetMessageW: %w", err)
		}
		if r1 == 0 {
			return nil
		}
		procTranslateMsg.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMsg.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func windowProc(hwnd uintptr, msg uint32, wParam uintptr, lParam uintptr) uintptr {
	a := getActiveApp()
	switch msg {
	case wmTray:
		if a != nil && isTrayActivation(lParam) {
			a.showMenu()
			return 0
		}
	case wmCommand:
		if a != nil {
			a.handleMenu(uint16(wParam & 0xffff))
			return 0
		}
	case wmClose:
		if a != nil {
			_, running := a.snapshot()
			if running {
				a.requestExit()
				return 0
			}
		}
		procDestroyWindow.Call(hwnd)
		return 0
	case wmDestroy:
		if a != nil {
			a.removeTrayIcon()
		}
		procPostQuitMessage.Call(0)
		return 0
	}
	ret, _, _ := procDefWindowProc.Call(hwnd, uintptr(msg), wParam, lParam)
	return ret
}

func isTrayActivation(lParam uintptr) bool {
	event := uint32(lParam & 0xffff)
	switch event {
	case wmLButtonUp, wmRButtonUp, wmLButtonDbl, wmRButtonDbl, wmContextMenu, ninSelect, ninKeySelect:
		return true
	case wmMouseMove:
		return false
	default:
		return false
	}
}

func postMessage(hwnd uintptr, msg uint32, wParam uintptr, lParam uintptr) {
	procPostMessage.Call(hwnd, uintptr(msg), wParam, lParam)
}

func setActiveApp(app *App) {
	activeMu.Lock()
	activeApp = app
	activeMu.Unlock()
}

func getActiveApp() *App {
	activeMu.Lock()
	defer activeMu.Unlock()
	return activeApp
}

type point struct {
	x int32
	y int32
}

type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
}

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

type guid struct {
	data1 uint32
	data2 uint16
	data3 uint16
	data4 [8]byte
}

type notifyIconData struct {
	cbSize            uint32
	hWnd              uintptr
	uID               uint32
	uFlags            uint32
	uCallbackMessage  uint32
	hIcon             uintptr
	szTip             [128]uint16
	dwState           uint32
	dwStateMask       uint32
	szInfo            [256]uint16
	uTimeoutOrVersion uint32
	szInfoTitle       [64]uint16
	dwInfoFlags       uint32
	guidItem          guid
	hBalloonIcon      uintptr
}
