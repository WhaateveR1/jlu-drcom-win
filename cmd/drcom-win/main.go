package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"jlu-drcom-win/internal/config"
	"jlu-drcom-win/internal/logging"
	"jlu-drcom-win/internal/runner"
	"jlu-drcom-win/internal/transport"
)

func main() {
	configPath := flag.String("config", "config.toml", "path to config.toml")
	loginOnly := flag.Bool("login-only", false, "run login flow and exit without heartbeat")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	logger := logging.New(cfg.DebugHexDump)
	logger.Info("config loaded", "server", cfg.ServerAddrString(), "bind", cfg.BindAddrString(), "adapter", cfg.AutoNetwork.InterfaceName)

	factory := func() (runner.Exchanger, error) {
		udpTransport, err := transport.NewTransport(cfg.BindUDPAddr(), cfg.ServerUDPAddr(), cfg.ReceiveTimeout)
		if err != nil {
			return nil, err
		}
		logger.Info("udp socket bound", "bind", cfg.BindAddrString(), "server", cfg.ServerAddrString())
		return udpTransport, nil
	}

	r := runner.NewWithTransportFactory(cfg, factory, rand.Reader, logger)
	defer r.Close()
	if *loginOnly {
		session, err := r.Login(context.Background())
		if err != nil {
			fmt.Fprintf(os.Stderr, "login failed: %v\n", err)
			os.Exit(1)
		}
		logger.Info("login flow completed", "state", r.State(), "server_indicator", fmt.Sprintf("%x", session.ServerDrcomIndicator))
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := r.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "runner failed: %v\n", err)
		os.Exit(1)
	}
	logger.Info("runner stopped", "state", r.State())
}
