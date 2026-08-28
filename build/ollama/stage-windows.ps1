# Stages the pinned Ollama release for the Windows installer.
#
# Downloads exactly the version internal/platform/ollamamanage.go demands and
# places ollama.exe, lib/, and the licence in build/ollama/windows-amd64/,
# which the NSIS installer copies to ollama/ beside the application. Prints
# the version the staged binary reports, so the record in PIN.md is a fact.
$ErrorActionPreference = 'Stop'

$source = Join-Path $PSScriptRoot '..\..\internal\platform\ollamamanage.go'
$match = Select-String -Path $source -Pattern 'PinnedOllamaVersion = "([^"]+)"'
if (-not $match) { throw "PinnedOllamaVersion not found in $source" }
$version = $match.Matches[0].Groups[1].Value

$dir = Join-Path $PSScriptRoot 'windows-amd64'
New-Item -ItemType Directory -Force $dir | Out-Null

$zip = Join-Path $env:TEMP ("ollama-windows-amd64-$version.zip")
if (-not (Test-Path $zip)) {
    Write-Host "downloading Ollama $version"
    Invoke-WebRequest -Uri "https://github.com/ollama/ollama/releases/download/v$version/ollama-windows-amd64.zip" -OutFile $zip
}

$work = Join-Path $env:TEMP ("ollama-stage-$version")
if (Test-Path $work) { Remove-Item -Recurse -Force $work }
Expand-Archive -Path $zip -DestinationPath $work

Copy-Item (Join-Path $work 'ollama.exe') $dir -Force
if (Test-Path (Join-Path $work 'lib')) {
    if (Test-Path (Join-Path $dir 'lib')) { Remove-Item -Recurse -Force (Join-Path $dir 'lib') }
    Copy-Item (Join-Path $work 'lib') $dir -Recurse -Force
}
# The release zip carries no licence file; it is fetched from the same tag.
Invoke-WebRequest -Uri "https://raw.githubusercontent.com/ollama/ollama/v/LICENSE" -OutFile (Join-Path  'LICENSE')

Write-Host "staged "
# Asked with no server to answer, so it reports its own version rather than
# that of whatever Ollama happens to be running on this machine.
:OLLAMA_HOST = '127.0.0.1:1'
& (Join-Path  'ollama.exe') --version 2>
