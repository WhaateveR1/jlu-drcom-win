# Release

This repository is intended to be published from the `jlu-drcom-win` directory.

## Source Repository

Do not commit local runtime files:

```text
config.toml
dist/
*.exe
*.log
.env
```

`config.example.toml` is the public template and should be committed.

## Local Check

Run from the repository root:

```powershell
go test ./...
.\scripts\build.ps1
```

The local release package is generated at:

```text
dist\jlu-drcom-win.zip
```

The zip should contain:

```text
CHANGELOG.md
config.example.toml
drcom-tray.exe
drcom-win.exe
LICENSE
NOTICE.md
README.md
USER_GUIDE.md
```

## GitHub Release

The GitHub Actions workflow builds and uploads the Windows package automatically.

```powershell
git tag v0.1.0
git push origin v0.1.0
```

Pushing a `v*` tag creates or updates the GitHub Release and uploads `jlu-drcom-win.zip`.
