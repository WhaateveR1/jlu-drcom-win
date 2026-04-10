//go:build !windows

package trayapp

import (
	"fmt"
	"io"
	"log/slog"

	"jlu-drcom-win/internal/config"
)

type App struct{}

func New(config.Config, string, io.Reader, *slog.Logger) *App {
	return &App{}
}

func (a *App) Run() error {
	return fmt.Errorf("tray app is only supported on Windows")
}
