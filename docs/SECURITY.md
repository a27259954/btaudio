# Security

BtAudio is a local desktop app with a small attack surface.

| Area | Behaviour |
|---|---|
| Network | None. BtAudio opens no ports, makes no network requests and has no telemetry or update check. |
| Bluetooth | Uses only Windows' own `AudioPlaybackConnection` API, and only with phones you have already paired in Windows Settings. It never pairs devices or changes Bluetooth settings. |
| Audio | Reads the level of the phone's stream for the meter. It never records or stores audio. |
| Data at rest | `%APPDATA%\BtAudio\config.json` holds the auto-connect choice per phone (keyed by the Windows device ID) and the UI language. Nothing else is written, apart from the WebView2 profile Windows creates for the window. |
| Startup | **Start with Windows** adds a value under `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`. Turning it off removes it. |
| Releases | Built by GitHub Actions from the tagged commit, with a SHA-256 checksum published next to each exe. Release builds are not code-signed yet. |

## Reporting a vulnerability

Please report security issues privately through GitHub's **Security → Report a vulnerability** on this repository, not in public issues.
