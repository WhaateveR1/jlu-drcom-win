# Development

## Ownership

- `cmd/drcom-tray`: one Windows GUI entrypoint; acquire the instance mutex before log rotation.
- `config`: standard TOML decoder, strict keys/types/bounds, consistent adapter/IP/MAC selection.
- `protocol`: pure packet builders and request-specific response matchers. No network or logging side effects.
- `transport`: one synchronous owner, fixed deadline for each exchange, peer filtering, context cancellation. Matchers decide packet semantics. No concurrent exchanges on one socket.
- `runner.Runner`: one session, validated login/heartbeats/logout. `Authenticated` is emitted only after the first heartbeat succeeds.
- `runner.Supervisor`: reload config/network, open/close transport, classify failures and bound reconnect backoff. No nested reconnect loop.
- `trayapp`: OS-thread-bound message loop. Worker state is posted back to the window; only the UI thread mutates the notification icon. Shutdown cleanup is bounded; resume preserves the user's login intent.
- `logging`: user-local rotating files; metadata only, never raw authentication packets.

## Verification

```powershell
go test ./...
go vet ./...
go test -race ./...  # Windows requires a compatible C compiler and CGO_ENABLED=1
go test ./internal/protocol -fuzz FuzzResponseMatcher -fuzztime 10s
.\scripts\test-build.ps1
.\scripts\build.ps1
```

Regression tests cover invalid heartbeats, truncated/wrong-phase replies, sequence wrap, duplicate/late UDP replies, foreign peers, interruptible reads and deadline cleanup, configuration reload/backoff, permanent errors, metadata-only logging, live rotation, TOML bounds and mixed adapter settings. The build-script tests use synthetic files and a fake Go command to exercise test/vet/build failures and protect user files and the last good zip.

## Observed Protocol Shapes

Sanitized JLU server headers observed on 2026-10-03 (payloads and credentials excluded):

| Request | Reply | Bytes | Additional checks |
|---|---|---:|---|
| Login/logout challenge `01` | `02` | 76 | Matching subtype; salt at 4..8 |
| Login `03` | `04` | 45 | Token at 23..39 |
| Keepalive auth `ff` | `07` | 72 | Opcode `06`; declared length 16 differs from datagram size |
| First heartbeat `07` | `07` | 272 | Opcode `0b`, phase `06`, matching sequence |
| Heartbeat step 1 | `07` | 40 | Opcode `0b`, phase `02`, matching sequence |
| Heartbeat step 2 | `07` | 40 | Opcode `0b`, phase `04`, matching sequence |
| Logout `06` | `04` | 25 | Distinct from the longer login success |

First/extra heartbeats also permit a normal phase-02 response. Do not interpret the keepalive's declared length as the full datagram size. The wire protocol lacks a universally usable transaction ID: challenge replies are matched by subtype, login by success type/required fields, keepalive by shape. This filters cross-phase duplicates but cannot prove freshness of every same-phase datagram. MD5 is required by this legacy protocol, not a modern password-storage choice.

## Release Safety

Build into a fresh GUID staging directory, package an explicit allowlist and replace the zip only after tests, vet and compilation succeed. Never clean or refresh an extracted runtime directory. Local config and logs are ignored and excluded from packaging. Source push runs CI; only a `v*` tag enables the separate write-permission release job.

OS integration tests do not replace live Windows testing. Reboot/logon timing, true sleep/resume, Explorer restart and adapter switching remain manual acceptance cases. Do not disrupt a remotely connected machine to claim those tests passed.
