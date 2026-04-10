package runner

import (
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"log/slog"
	"sync/atomic"
	"time"

	"jlu-drcom-win/internal/config"
	"jlu-drcom-win/internal/logging"
	"jlu-drcom-win/internal/protocol"
	"jlu-drcom-win/internal/transport"
)

type Runner struct {
	cfg       config.Config
	factory   TransportFactory
	exchanger Exchanger
	rng       io.Reader
	logger    *slog.Logger
	state     atomic.Value
}

type Exchanger interface {
	Exchange(packet []byte) ([]byte, error)
}

type TransportFactory func() (Exchanger, error)

func New(cfg config.Config, exchanger Exchanger, rng io.Reader, logger *slog.Logger) *Runner {
	r := newBaseRunner(cfg, rng, logger)
	r.exchanger = exchanger
	return r
}

func NewWithTransportFactory(cfg config.Config, factory TransportFactory, rng io.Reader, logger *slog.Logger) *Runner {
	r := newBaseRunner(cfg, rng, logger)
	r.factory = factory
	return r
}

func newBaseRunner(cfg config.Config, rng io.Reader, logger *slog.Logger) *Runner {
	if rng == nil {
		rng = rand.Reader
	}
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	r := &Runner{
		cfg:    cfg,
		rng:    rng,
		logger: logger,
	}
	r.state.Store(StateDisconnected)
	return r
}

func (r *Runner) State() State {
	if state, ok := r.state.Load().(State); ok {
		return state
	}
	return StateFailed
}

func (r *Runner) setState(state State) {
	r.state.Store(state)
}

func (r *Runner) Login(ctx context.Context) (protocol.Session, error) {
	if err := r.ensureTransport(); err != nil {
		return protocol.Session{}, err
	}
	var session protocol.Session
	protocolConfig := r.cfg.ProtocolConfig()

	r.setState(StateLoginChallenge)
	r.logger.Info("login challenge started")
	challenge := protocol.BuildLoginChallenge(protocolConfig.AuthVersion, r.rng)
	r.dumpPacket("login challenge request", challenge, nil)

	var challengeResponse []byte
	if err := r.exchangeWithRetry(ctx, func() error {
		var err error
		challengeResponse, err = r.exchanger.Exchange(challenge)
		return err
	}); err != nil {
		r.setState(StateFailed)
		return session, fmt.Errorf("login challenge exchange: %w", err)
	}
	r.dumpPacket("login challenge response", challengeResponse, nil)

	salt, err := protocol.ParseLoginChallengeResponse(challengeResponse)
	if err != nil {
		r.setState(StateFailed)
		return session, fmt.Errorf("parse login challenge response: %w", err)
	}
	session.LoginSalt = salt
	r.logger.Info("login challenge succeeded")

	r.setState(StateLoggingIn)
	loginPacket := protocol.BuildLoginPacket(protocolConfig, &session, r.rng)
	r.dumpPacket("login request redacted", loginPacket, loginRedactions(len(protocolConfig.Password)))

	var loginResponse []byte
	if err := r.exchangeWithRetry(ctx, func() error {
		var err error
		loginResponse, err = r.exchanger.Exchange(loginPacket)
		return err
	}); err != nil {
		r.setState(StateFailed)
		return session, fmt.Errorf("login exchange: %w", err)
	}
	r.dumpPacket("login response", loginResponse, nil)

	if err := protocol.ParseLoginResponse(loginResponse, &session); err != nil {
		r.setState(StateFailed)
		return session, fmt.Errorf("parse login response: %w", err)
	}
	r.setState(StateOnline)
	r.logger.Info("login succeeded")
	return session, nil
}

func (r *Runner) Run(ctx context.Context) error {
	session, err := r.Login(ctx)
	if err != nil {
		return err
	}

	ticker := time.NewTicker(r.cfg.HeartbeatInterval)
	defer ticker.Stop()
	for {
		if err := r.HeartbeatOnce(ctx, &session); err != nil {
			if ctx.Err() != nil {
				return r.logoutForShutdown(&session)
			}
			r.logger.Warn("heartbeat failed; reconnecting", "error", err)
			if err := r.Reconnect(ctx, &session); err != nil {
				return err
			}
			continue
		}

		select {
		case <-ctx.Done():
			return r.logoutForShutdown(&session)
		case <-ticker.C:
		}
	}
}

func (r *Runner) Logout(ctx context.Context, session *protocol.Session) error {
	if err := r.ensureTransport(); err != nil {
		return err
	}
	protocolConfig := r.cfg.ProtocolConfig()
	r.setState(StateLoggingOut)
	r.logger.Info("logout challenge started")

	challenge := protocol.BuildLogoutChallenge(protocolConfig.AuthVersion, r.rng)
	r.dumpPacket("logout challenge request", challenge, nil)

	var challengeResponse []byte
	if err := r.exchangeWithRetry(ctx, func() error {
		var err error
		challengeResponse, err = r.exchanger.Exchange(challenge)
		return err
	}); err != nil {
		r.setState(StateFailed)
		return fmt.Errorf("logout challenge exchange: %w", err)
	}
	r.dumpPacket("logout challenge response", challengeResponse, nil)

	salt, err := protocol.ParseLogoutChallengeResponse(challengeResponse)
	if err != nil {
		r.setState(StateFailed)
		return fmt.Errorf("parse logout challenge response: %w", err)
	}
	session.LogoutSalt = salt
	r.logger.Info("logout challenge succeeded")

	logoutPacket := protocol.BuildLogoutPacket(protocolConfig, *session)
	r.dumpPacket("logout request redacted", logoutPacket, loginRedactions(len(protocolConfig.Password)))

	var logoutResponse []byte
	if err := r.exchangeWithRetry(ctx, func() error {
		var err error
		logoutResponse, err = r.exchanger.Exchange(logoutPacket)
		return err
	}); err != nil {
		r.setState(StateFailed)
		return fmt.Errorf("logout exchange: %w", err)
	}
	r.dumpPacket("logout response", logoutResponse, nil)
	if err := protocol.ParseLogoutResponse(logoutResponse); err != nil {
		r.setState(StateFailed)
		return fmt.Errorf("parse logout response: %w", err)
	}

	r.setState(StateStopped)
	r.logger.Info("logout succeeded")
	return nil
}

func (r *Runner) Reconnect(ctx context.Context, session *protocol.Session) error {
	r.setState(StateReconnecting)
	attempts := r.cfg.RetryCount + 1
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		r.logger.Warn("reconnect attempt started", "attempt", attempt, "max_attempts", attempts)
		if r.factory != nil {
			r.closeTransport()
			if err := r.openTransport(); err != nil {
				lastErr = err
				r.logger.Warn("reopen udp transport failed", "error", err)
				continue
			}
		}
		newSession, err := r.Login(ctx)
		if err != nil {
			lastErr = err
			r.logger.Warn("reconnect login failed", "error", err)
			continue
		}
		*session = newSession
		r.logger.Info("reconnect login succeeded")
		return nil
	}
	r.setState(StateFailed)
	return fmt.Errorf("reconnect failed after %d attempt(s): %w", attempts, lastErr)
}

func (r *Runner) HeartbeatOnce(ctx context.Context, session *protocol.Session) error {
	protocolConfig := r.cfg.ProtocolConfig()
	if err := r.KeepAliveAuth(ctx, *session, time.Now()); err != nil {
		return err
	}

	if session.HeartbeatCount == 0 {
		packet := protocol.BuildFirstHeartbeat(protocolConfig, *session, r.rng)
		r.dumpPacket("first heartbeat request", packet, nil)
		if err := r.exchangeWithRetry(ctx, func() error {
			response, err := r.exchanger.Exchange(packet)
			if err != nil {
				return err
			}
			r.dumpPacket("first heartbeat response", response, nil)
			return protocol.ParseHeartbeatAck(response)
		}); err != nil {
			return fmt.Errorf("first heartbeat exchange: %w", err)
		}
		session.HeartbeatCount++
	}

	if session.HeartbeatCount%21 == 0 {
		packet := protocol.BuildExtraHeartbeat(protocolConfig, *session, r.rng)
		r.dumpPacket("extra heartbeat request", packet, nil)
		if err := r.exchangeWithRetry(ctx, func() error {
			response, err := r.exchanger.Exchange(packet)
			if err != nil {
				return err
			}
			r.dumpPacket("extra heartbeat response", response, nil)
			return protocol.ParseHeartbeatAck(response)
		}); err != nil {
			return fmt.Errorf("extra heartbeat exchange: %w", err)
		}
		session.HeartbeatCount++
	}

	step1 := protocol.BuildHeartbeatStep1(protocolConfig, *session, r.rng)
	var randomToken [4]byte
	copy(randomToken[:], step1[8:12])
	r.dumpPacket("heartbeat step1 request", step1, nil)
	if err := r.exchangeWithRetry(ctx, func() error {
		response, err := r.exchanger.Exchange(step1)
		if err != nil {
			return err
		}
		r.dumpPacket("heartbeat step1 response", response, nil)
		return protocol.ParseHeartbeatStep1Response(response, session)
	}); err != nil {
		return fmt.Errorf("heartbeat step1 exchange: %w", err)
	}
	session.HeartbeatCount++

	step2 := protocol.BuildHeartbeatStep2(protocolConfig, *session, randomToken)
	r.dumpPacket("heartbeat step2 request", step2, nil)
	if err := r.exchangeWithRetry(ctx, func() error {
		response, err := r.exchanger.Exchange(step2)
		if err != nil {
			return err
		}
		r.dumpPacket("heartbeat step2 response", response, nil)
		return protocol.ParseHeartbeatAck(response)
	}); err != nil {
		return fmt.Errorf("heartbeat step2 exchange: %w", err)
	}
	session.HeartbeatCount++
	r.logger.Info("heartbeat succeeded", "count", session.HeartbeatCount)
	return nil
}

func (r *Runner) KeepAliveAuth(ctx context.Context, session protocol.Session, now time.Time) error {
	packet := protocol.BuildKeepAliveAuth(session, now)
	r.dumpPacket("keepalive auth request", packet, nil)
	if err := r.exchangeWithRetry(ctx, func() error {
		response, err := r.exchanger.Exchange(packet)
		if err != nil {
			return err
		}
		r.dumpPacket("keepalive auth response", response, nil)
		return protocol.ParseKeepAliveAuthResponse(response)
	}); err != nil {
		return fmt.Errorf("keepalive auth exchange: %w", err)
	}
	return nil
}

func (r *Runner) Close() error {
	return r.closeTransport()
}

func (r *Runner) ensureTransport() error {
	if r.exchanger != nil {
		return nil
	}
	return r.openTransport()
}

func (r *Runner) openTransport() error {
	if r.factory == nil {
		return fmt.Errorf("runner transport is nil")
	}
	exchanger, err := r.factory()
	if err != nil {
		return err
	}
	if exchanger == nil {
		return fmt.Errorf("transport factory returned nil transport")
	}
	r.exchanger = exchanger
	return nil
}

func (r *Runner) closeTransport() error {
	if r.exchanger == nil {
		return nil
	}
	closer, ok := r.exchanger.(interface{ Close() error })
	r.exchanger = nil
	if !ok {
		return nil
	}
	if err := closer.Close(); err != nil {
		r.logger.Warn("close transport failed", "error", err)
		return err
	}
	return nil
}

func (r *Runner) logoutForShutdown(session *protocol.Session) error {
	timeout := r.cfg.ReceiveTimeout * time.Duration(r.cfg.RetryCount+2) * 2
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := r.Logout(ctx, session); err != nil {
		r.closeTransport()
		return err
	}
	return r.closeTransport()
}

func (r *Runner) exchangeWithRetry(ctx context.Context, fn func() error) error {
	attempts := r.cfg.RetryCount + 1
	return transport.RetryExchange(ctx, attempts, func() error {
		err := fn()
		if err != nil {
			r.logger.Warn("udp exchange failed", "error", err)
		}
		return err
	})
}

func (r *Runner) dumpPacket(name string, packet []byte, redactions []logging.ByteRange) {
	if !r.cfg.DebugHexDump {
		return
	}
	dump := logging.HexDump(packet)
	if len(redactions) > 0 {
		dump = logging.HexDumpRedacted(packet, redactions...)
	}
	r.logger.Debug(name, "length", len(packet), "hex", "\n"+dump)
}

func loginRedactions(passwordLen int) []logging.ByteRange {
	rorLen := min(passwordLen, 16)
	checksumStart := 316 + rorLen
	return []logging.ByteRange{
		{Start: 4, End: 20},
		{Start: 58, End: 64},
		{Start: 64, End: 80},
		{Start: 314, End: 314 + rorLen},
		{Start: checksumStart, End: checksumStart + 4},
		{Start: 322 + rorLen, End: 328 + rorLen},
	}
}
