//go:build !windows

package trayapp

import (
	"fmt"
	"log/slog"
)

type App struct{}

func (a *App) SetAutoLogin(bool) {}

func ConfigureStartup(string, string) error {
	return fmt.Errorf("startup is only supported on Windows")
}

func New(string, string, *slog.Logger) *App {
	return &App{}
}

func AcquireInstance() (func(), bool, error) { return func() {}, false, nil }
func ShowError(message string)               { fmt.Println(message) }

func (a *App) Run() error {
	return fmt.Errorf("tray app is only supported on Windows")
}
