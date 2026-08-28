# Bundling Ollama

The app prefers an Ollama the recruiter already runs (`http://localhost:11434`).
When none answers, it launches its own copy from `ollama/ollama[.exe]` beside
the application binary, pinned to `127.0.0.1:11435`, with model weights under
the user cache (`talent-hound/ollama-models`). Nothing bundled and nothing
running means the first-run wizard's Ollama step explains, as before.

## Vendoring the binary

The binary is not in the repository. To produce a bundled build, stage the
official release into an untracked folder first:

- macOS: download the darwin release from https://github.com/ollama/ollama/releases,
  place the standalone `ollama` binary (and nothing else) in
  `build/ollama/darwin-arm64/`.
- Windows: run `just ollama-windows`. It downloads the pinned release
  (`platform.PinnedOllamaVersion`, recorded in `build/ollama/PIN.md`) and
  stages `ollama.exe`, `lib/`, and the licence in `build/ollama/windows-amd64/`.
  The NSIS installer copies that folder to `ollama/` beside the application
  whenever it is present; built without it, the installer relies on detection
  and the first-run wizard says how to install Ollama. The MSIX package does
  not carry it yet.

Ollama is MIT-licensed; include its LICENSE file in the staged folder.

## Development override

`TALENT_HOUND_OLLAMA_PATH=/path/to/ollama` points the app at any binary,
matching `TALENT_HOUND_SIDECAR_PATH` for the extraction sidecar.
