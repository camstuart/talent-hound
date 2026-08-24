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
- Windows: place `ollama.exe` and the `lib/` folder from the
  `ollama-windows-amd64.zip` release in `build/ollama/windows-amd64/`, and add
  a copy step mirroring the darwin one to the NSIS/MSIX packaging when Windows
  packaging is next touched (`build/windows/Taskfile.yml`). Until then the
  Windows installer ships without a bundled copy and relies on detection.

Ollama is MIT-licensed; include its LICENSE file in the staged folder.

## Development override

`TALENT_HOUND_OLLAMA_PATH=/path/to/ollama` points the app at any binary,
matching `TALENT_HOUND_SIDECAR_PATH` for the extraction sidecar.
