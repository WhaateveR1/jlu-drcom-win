package runner

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync/atomic"
	"time"

	"jlu-drcom-win/internal/config"
	"jlu-drcom-win/internal/protocol"
	"jlu-drcom-win/internal/transport"
)

var ErrAuthenticationRejected = errors.New("authentication rejected; check account and campus network")

type Exchanger interface {
	Exchange(context.Context, []byte, func([]byte) (bool, error)) ([]byte, error)
}

// Runner owns one session. Supervisor reloads the adapter/IP before reconnecting.
type Runner struct {
	cfg       config.Config
	exchanger Exchanger
	rng       io.Reader
	logger    *slog.Logger
	state     atomic.Value
	OnState   func(State)
}

func New(cfg config.Config, exchanger Exchanger, rng io.Reader, logger *slog.Logger) *Runner {
	if rng == nil {
		rng = rand.Reader
	}
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	r := &Runner{cfg: cfg, exchanger: exchanger, rng: rng, logger: logger}
	r.state.Store(StateDisconnected)
	return r
}

func (r *Runner) State() State { return r.state.Load().(State) }
func (r *Runner) setState(s State) {
	r.state.Store(s)
	if r.OnState != nil {
		r.OnState(s)
	}
}

func (r *Runner) exchange(ctx context.Context, name string, packet []byte) ([]byte, error) {
	var response []byte
	err := transport.RetryExchange(ctx, r.cfg.RetryCount+1, func() error {
		var err error
		response, err = r.exchanger.Exchange(ctx, packet, protocol.ResponseMatcher(packet))
		if err == nil {
			ok, matchErr := protocol.ResponseMatcher(packet)(response)
			if matchErr != nil {
				err = matchErr
			} else if !ok {
				err = fmt.Errorf("unexpected response")
			}
		}
		if r.cfg.DebugHexDump {
			r.logger.Debug("protocol exchange", "phase", name, "request_bytes", len(packet), "response_bytes", len(response), "error", err)
		}
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return response, nil
}

func (r *Runner) Login(ctx context.Context) (protocol.Session, error) {
	var s protocol.Session
	cfg := r.cfg.ProtocolConfig()
	r.setState(StateLoginChallenge)
	p, err := r.exchange(ctx, "login challenge", protocol.BuildLoginChallenge(cfg.AuthVersion, r.rng))
	if err != nil {
		return s, err
	}
	s.LoginSalt, err = protocol.ParseLoginChallengeResponse(p)
	if err != nil {
		return s, err
	}
	r.setState(StateLoggingIn)
	p, err = r.exchange(ctx, "login", protocol.BuildLoginPacket(cfg, &s, r.rng))
	if errors.Is(err, protocol.ErrRejected) {
		return s, ErrAuthenticationRejected
	}
	if err != nil {
		return s, err
	}
	if err = protocol.ParseLoginResponse(p, &s); err != nil {
		return s, err
	}
	r.logger.Info("login succeeded")
	return s, nil
}

func (r *Runner) Run(ctx context.Context) error {
	if r.cfg.HeartbeatInterval <= 0 {
		return fmt.Errorf("heartbeat interval must be positive")
	}
	s, err := r.Login(ctx)
	if err != nil {
		r.setState(StateFailed)
		return err
	}
	defer func() {
		if ctx.Err() != nil {
			logoutCtx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
			defer cancel()
			if err := r.Logout(logoutCtx, &s); err != nil {
				r.logger.Warn("best-effort logout failed", "error", err)
			}
		}
	}()
	for {
		if err := r.HeartbeatOnce(ctx, &s); err != nil {
			r.setState(StateFailed)
			return err
		}
		r.setState(StateOnline)
		timer := time.NewTimer(r.cfg.HeartbeatInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (r *Runner) Logout(ctx context.Context, s *protocol.Session) error {
	r.setState(StateLoggingOut)
	cfg := r.cfg.ProtocolConfig()
	p, err := r.exchange(ctx, "logout challenge", protocol.BuildLogoutChallenge(cfg.AuthVersion, r.rng))
	if err != nil {
		return err
	}
	s.LogoutSalt, err = protocol.ParseLogoutChallengeResponse(p)
	if err != nil {
		return err
	}
	if _, err := r.exchange(ctx, "logout", protocol.BuildLogoutPacket(cfg, *s)); err != nil {
		return err
	}
	r.setState(StateStopped)
	r.logger.Info("logout succeeded")
	return nil
}

func (r *Runner) HeartbeatOnce(ctx context.Context, s *protocol.Session) error {
	cfg := r.cfg.ProtocolConfig()
	if err := r.KeepAliveAuth(ctx, *s, time.Now()); err != nil {
		return err
	}
	if s.HeartbeatCount == 0 || s.HeartbeatCount%21 == 0 {
		var packet []byte
		if s.HeartbeatCount == 0 {
			packet = protocol.BuildFirstHeartbeat(cfg, *s, r.rng)
		} else {
			packet = protocol.BuildExtraHeartbeat(cfg, *s, r.rng)
		}
		if _, err := r.exchange(ctx, "first/extra heartbeat", packet); err != nil {
			return err
		}
		s.HeartbeatCount++
	}
	step1 := protocol.BuildHeartbeatStep1(cfg, *s, r.rng)
	var randomToken [4]byte
	copy(randomToken[:], step1[8:12])
	p, err := r.exchange(ctx, "heartbeat step1", step1)
	if err != nil {
		return err
	}
	if err := protocol.ParseHeartbeatStep1Response(p, s); err != nil {
		return err
	}
	s.HeartbeatCount++
	if _, err := r.exchange(ctx, "heartbeat step2", protocol.BuildHeartbeatStep2(cfg, *s, randomToken)); err != nil {
		return err
	}
	s.HeartbeatCount++
	r.logger.Info("heartbeat succeeded", "count", s.HeartbeatCount)
	return nil
}

func (r *Runner) KeepAliveAuth(ctx context.Context, s protocol.Session, now time.Time) error {
	_, err := r.exchange(ctx, "keepalive auth", protocol.BuildKeepAliveAuth(s, now))
	return err
}

func (r *Runner) Close() error {
	if closer, ok := r.exchanger.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}
