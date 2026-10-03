package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"jlu-drcom-win/internal/logging"
	"jlu-drcom-win/internal/trayapp"
)

func main() {
	if err := run(); err != nil {
		trayapp.ShowError(err.Error())
		os.Exit(1)
	}
}

func run() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	configPath := flag.String("config", filepath.Join(filepath.Dir(exe), "config.toml"), "configuration path")
	autoLogin := flag.Bool("autologin", true, "authenticate on launch")
	startup := flag.String("startup", "", "enable or disable current-user logon task")
	flag.Parse()
	abs, err := filepath.Abs(*configPath)
	if err != nil {
		return err
	}
	if *startup != "" {
		return trayapp.ConfigureStartup(*startup, abs)
	}
	release, alreadyRunning, err := trayapp.AcquireInstance()
	if err != nil {
		return err
	}
	defer release()
	if alreadyRunning {
		return nil
	}
	logger, closer, logDir, err := logging.UserLogger()
	if err != nil {
		return fmt.Errorf("cannot open user log folder: %w", err)
	}
	defer closer.Close()
	logger.Info("tray starting", "autologin", *autoLogin)
	app := trayapp.New(abs, logDir, logger)
	app.SetAutoLogin(*autoLogin)
	if err := app.Run(); err != nil {
		logger.Error("tray failed", "error", err)
		return err
	}
	logger.Info("tray stopped")
	return nil
}
