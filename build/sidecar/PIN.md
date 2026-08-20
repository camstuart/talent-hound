# MarkItDown sidecar pin

The extraction sidecar is a PyInstaller **one-dir** package of the MarkItDown
CLI. The package itself is a build artifact and is not committed; this file is
the committed record of what was built.

| Field | Value |
| --- | --- |
| MarkItDown | 0.1.2 (see `requirements.txt`) |
| PyInstaller | 6.11.1 |
| Python | _the `python` line printed by `just sidecar`_ |
| Built on | _date_ |
| Executable SHA-256 | _the `sha256` line printed by `just sidecar`_ |

Rebuild with `just sidecar` (Windows); it prints every value above. Plugins are off unless `--use-plugins`
is passed, and the app never passes it; no network converters are enabled.
