# Notices

## Acknowledgement

This project references protocol fields and behavior from the original C implementation:

- AndrewLawrence80/jlu-drcom-client: https://github.com/AndrewLawrence80/jlu-drcom-client

The Windows client in this repository is a Go rewrite. It keeps the protocol knowledge, packet fields, byte offsets, and checksum behavior needed for compatibility, while replacing the original runtime model with a single-owner UDP state machine, explicit timeouts, retry handling, reconnect, logout, and a Windows tray shell.
