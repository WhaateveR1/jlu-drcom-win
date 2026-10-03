package runner

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"jlu-drcom-win/internal/config"
	"jlu-drcom-win/internal/transport"
)

type Supervisor struct {
	Load    func() (config.Config, error)
	Open    func(config.Config) (Exchanger, error)
	Logger  *slog.Logger
	OnState func(State)
	Wait    func(context.Context, time.Duration) error
}

func (s *Supervisor) Run(ctx context.Context) error {
	logger := s.Logger
	if logger == nil {
		logger = slog.Default()
	}
	open := s.Open
	if open == nil {
		open = func(c config.Config) (Exchanger, error) {
			return transport.NewTransport(c.BindUDPAddr(), c.ServerUDPAddr(), c.ReceiveTimeout)
		}
	}
	wait := s.Wait
	if wait == nil {
		wait = waitForRetry
	}
	delay := 5 * time.Second
	for ctx.Err() == nil {
		cfg, err := s.Load()
		if err != nil && !errors.Is(err, config.ErrNetworkUnavailable) {
			return err
		}
		if err == nil {
			var tr Exchanger
			tr, err = open(cfg)
			if err == nil {
				logger.Info("udp socket bound", "bind", cfg.BindAddrString(), "adapter", cfg.AutoNetwork.InterfaceName)
				r := New(cfg, tr, nil, logger)
				r.OnState = func(state State) {
					if state == StateOnline {
						delay = 5 * time.Second
					}
					if s.OnState != nil {
						s.OnState(state)
					}
				}
				err = r.Run(ctx)
				_ = r.Close()
			}
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errors.Is(err, ErrAuthenticationRejected) {
			return err
		}
		logger.Warn("authentication unavailable; retry scheduled", "delay", delay, "error", err)
		if s.OnState != nil {
			s.OnState(StateReconnecting)
		}
		if err := wait(ctx, delay); err != nil {
			return err
		}
		delay = min(delay*2, 60*time.Second)
	}
	return ctx.Err()
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
