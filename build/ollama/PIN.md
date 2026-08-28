# Bundled Ollama pin

A packaged build carries its own copy of Ollama beside the application, in
`ollama/`, so the recruiter installs nothing else. The binaries are a build
input and are not committed (`build/ollama/<platform>/` is ignored); this
file is the committed record of what is staged.

| Field | Value |
| --- | --- |
| Ollama | 0.33.1 (`platform.PinnedOllamaVersion`) |
| Windows | `ollama.exe`, `lib/`, `LICENSE` from `ollama-windows-amd64.zip` |
| macOS | the standalone `ollama` binary from the darwin release |
| Licence | MIT — the release's LICENSE file is staged beside the binary |

Stage Windows with `just ollama-windows`; it downloads exactly the pinned
release and prints the version the staged binary reports. The packaging test
refuses a staged binary that reports another version, and the installer only
carries the folder when it is present — built without it, the application
relies on detecting an Ollama the recruiter installed, as before.

Changing the pin is a one-line change to `PinnedOllamaVersion`, then this
record, then re-staging.
