# Builds the pinned MarkItDown PyInstaller one-dir sidecar into build/sidecar/dist.
# Run on Windows: just sidecar
$ErrorActionPreference = "Stop"
$here = Split-Path -Parent $MyInvocation.MyCommand.Path
$venv = Join-Path $here ".venv"

if (-not (Test-Path $venv)) { python -m venv $venv }
& (Join-Path $venv "Scripts\python.exe") -m pip install --upgrade pip
& (Join-Path $venv "Scripts\pip.exe") install -r (Join-Path $here "requirements.txt")

& (Join-Path $venv "Scripts\pyinstaller.exe") `
    --noconfirm --clean --onedir --name markitdown-sidecar `
    --distpath (Join-Path $here "dist") --workpath (Join-Path $here "work") `
    --specpath $here `
    (Join-Path $venv "Scripts\markitdown.exe")

$exe = Join-Path $here "dist\markitdown-sidecar\markitdown-sidecar.exe"
Write-Host "sidecar: $exe"
Write-Host "sha256 : $((Get-FileHash $exe -Algorithm SHA256).Hash)"
Write-Host "python : $(& (Join-Path $venv 'Scripts\python.exe') --version)"
Write-Host "Record these in build/sidecar/PIN.md, then set TH_SIDECAR_EXE=$exe"
