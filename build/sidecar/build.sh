#!/usr/bin/env bash
# Builds the pinned MarkItDown PyInstaller one-dir sidecar into build/sidecar/dist.
# Run on macOS/Linux: just sidecar  (Windows uses build.ps1)
set -euo pipefail
here="$(cd "$(dirname "$0")" && pwd)"
venv="$here/.venv"

# Python pinned like everything else here: the sidecar digest in PIN.md is
# only reproducible when the interpreter is the same.
uv venv --quiet --python 3.13 "$venv"
uv pip install --quiet --python "$venv/bin/python" -r "$here/requirements.txt"

# The pinned PyInstaller's hooks predate numpy 2 (private numpy._core
# submodules load dynamically) and know nothing of magika's bundled model
# files; collect both or the binary dies on import.
"$venv/bin/pyinstaller" \
  --noconfirm --clean --onedir --name markitdown-sidecar \
  --collect-submodules numpy \
  --collect-data magika \
  --distpath "$here/dist" --workpath "$here/work" --specpath "$here" \
  "$venv/bin/markitdown"

exe="$here/dist/markitdown-sidecar/markitdown-sidecar"
echo "sidecar: $exe"
echo "sha256 : $(shasum -a 256 "$exe" | cut -d' ' -f1)"
echo "python : $("$venv/bin/python" --version)"
echo "Record these in build/sidecar/PIN.md."
