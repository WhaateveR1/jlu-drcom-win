# Changelog

## v0.1.0

- Added Windows tray client with Login, Logout, Exit, status display, and current-user startup toggle.
- Added command-line client for debugging and log inspection.
- Added automatic IPv4 and MAC detection for the active physical adapter.
- Added manual adapter override through `adapter_hint`, `ip`, and `mac`.
- Added login, keepalive auth, heartbeat, logout, timeout retry, and reconnect flows.
- Added protocol packet builders and unit tests.
- Added mock UDP integration tests.
- Added Windows build script and GitHub Actions workflow.
