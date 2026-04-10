# Development Notes

## Architecture

```text
cmd/
  drcom-win/      CLI entrypoint
  drcom-tray/     Windows tray entrypoint

internal/
  config/         config.toml parsing and validation
  protocol/       packet builders, parsers, checksums
  transport/      UDP socket, timeout, source validation
  runner/         login, heartbeat, logout, reconnect state machine
  trayapp/        Windows tray shell
  logging/        slog and hex dump helpers
  session/        session type alias
```

Protocol code has no socket, sleep, or logging dependency. Transport code does not understand Dr.COM packet semantics. Runner is the only state owner.

## Build

```powershell
go test ./...
.\scripts\build.ps1
```

Build output:

```text
dist\jlu-drcom-win\drcom-win.exe
dist\jlu-drcom-win\drcom-tray.exe
dist\jlu-drcom-win\config.example.toml
dist\jlu-drcom-win\README.md
dist\jlu-drcom-win\USER_GUIDE.md
dist\jlu-drcom-win.zip
```

The build script stages files in a temporary directory first. This keeps the zip current even when an old exe in `dist\jlu-drcom-win` is still running and cannot be overwritten.

## Tests

Current tests cover:

- Config parsing and validation.
- Automatic IPv4 and MAC detection, including virtual adapter avoidance and adapter hints.
- Challenge packet construction and parsing.
- Login packet key offsets and checksums.
- Keepalive auth and heartbeat packet fields.
- Logout packet fields.
- UDP exchange, timeout, and retry.
- Runner login, heartbeat, logout, reconnect, and cancel-to-logout behavior.
- Mock UDP login plus one heartbeat round.

## Local Secrets

`config.toml` is ignored by git and is not copied into the release zip. Only `config.example.toml` is packaged.
