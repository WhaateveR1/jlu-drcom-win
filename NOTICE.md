# Notices

## Acknowledgement

This project references protocol fields and behavior from the original C implementation:

- AndrewLawrence80/jlu-drcom-client: https://github.com/AndrewLawrence80/jlu-drcom-client

The Windows client in this repository is a Go rewrite. It keeps the protocol knowledge, packet fields, byte offsets, and checksum behavior needed for compatibility, while replacing the original runtime model with a single-owner UDP state machine, explicit timeouts, retry handling, reconnect, logout, and a Windows tray shell.

## Third-Party Software

The binary includes github.com/BurntSushi/toml, licensed under the MIT License:

Copyright (c) 2013 TOML authors

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
