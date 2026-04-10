package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"jlu-drcom-win/internal/config"
	"jlu-drcom-win/internal/logging"
	"jlu-drcom-win/internal/trayapp"
)

func main() {
	configPath := flag.String("config", "config.toml", "path to config.toml")
	flag.Parse()

	absConfigPath, err := filepath.Abs(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve config path: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.Load(absConfigPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	logger := logging.New(cfg.DebugHexDump)
	logger.Info("config loaded", "server", cfg.ServerAddrString(), "bind", cfg.BindAddrString(), "adapter", cfg.AutoNetwork.InterfaceName)
	app := trayapp.New(cfg, absConfigPath, rand.Reader, logger)
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "tray app failed: %v\n", err)
		os.Exit(1)
	}
}
