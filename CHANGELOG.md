# Changelog

## Unreleased

- Ship only the tray client; remove the prototype CLI and unused session alias.
- Match UDP responses by request/phase/sequence, discard unrelated replies within a fixed deadline, and interrupt I/O on cancellation.
- Verify heartbeat and logout responses against sanitized JLU server shapes; report authentication only after a successful heartbeat.
- Centralize reconnect ownership with fresh adapter/config discovery, bounded backoff and permanent-error handling.
- Handle Windows session end and suspend/resume; marshal tray updates to the UI thread.
- Store metadata-only logs in LocalAppData and rotate while running; acquire the instance lock before logging.
- Use a standard TOML parser with strict keys/types, bounded timing values and consistent IP/MAC/bind settings.
- Preserve all user runtime files and the previous package on failed builds; add packaging, network, lifecycle and secrecy regression tests.

- Start the tray at user logon through Task Scheduler, with automatic login and migration from the legacy Run key.
- Keep the tray responsive while waiting for an available physical adapter; retry failed sessions and reload network configuration.
- Bind authentication UDP to the selected IPv4 and Windows interface, excluding known virtual adapters from automatic fallback.
- Reject failed or unrelated login replies instead of treating any sufficiently long packet as success.
- Show Authenticated for protocol success, retain a tray log, and prevent duplicate tray instances.
- Document DNS versus authentication failures and provide an explicit, reversible campus DNS repair script.

## v0.1.0

- Added Windows tray client with Login, Logout, Exit, status display, and current-user startup toggle.
- Added command-line client for debugging and log inspection.
- Added automatic IPv4 and MAC detection for the active physical adapter.
- Added manual adapter override through `adapter_hint`, `ip`, and `mac`.
- Added login, keepalive auth, heartbeat, logout, timeout retry, and reconnect flows.
- Added protocol packet builders and unit tests.
- Added mock UDP integration tests.
- Added Windows build script and GitHub Actions workflow.
