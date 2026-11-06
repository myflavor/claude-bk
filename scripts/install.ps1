# Install claude-bk from GitHub Releases
# Usage: irm https://raw.githubusercontent.com/myflavor/claude-bk/main/scripts/install.ps1 | iex
#
# Downloads the per-platform binary (claude-bk-windows-<arch>.exe) and places
# it next to claude.
$ErrorActionPreference = "Stop"

$Repo = "myflavor/claude-bk"

# Detect arch, map to the release binary name
$Arch = $env:PROCESSOR_ARCHITECTURE
switch ($Arch) {
    "AMD64" { $Bin = "claude-bk-windows-amd64.exe" }
    "ARM64" { $Bin = "claude-bk-windows-arm64.exe" }
    default { Write-Error "Unsupported architecture: $Arch"; exit 1 }
}

# Find claude location (binary installs next to it)
$ClaudeCmd = Get-Command claude -ErrorAction SilentlyContinue
if (-not $ClaudeCmd) {
    Write-Error "Error: claude command not found in PATH"
    Write-Error "Please install Claude Code first: https://code.claude.com"
    exit 1
}

$ClaudeDir = Split-Path -Parent $ClaudeCmd.Source
$BkTarget = Join-Path $ClaudeDir "claude-bk.exe"

$TmpBin = [System.IO.Path]::GetTempFileName() + ".exe"
$BkStaged = $null
try {
    $Url = "https://github.com/$Repo/releases/latest/download/$Bin"
    Write-Host "Downloading $Bin from $Repo..."
    Invoke-WebRequest -Uri $Url -OutFile $TmpBin -UseBasicParsing
    if ((Get-Item $TmpBin).Length -eq 0) {
        throw "Downloaded file is empty: $Bin"
    }

    # Atomic install: stage in the target dir, then rename (same volume).
    $BkStaged = Join-Path $ClaudeDir ".claude-bk.exe.tmp"
    Copy-Item -Force $TmpBin $BkStaged
    Move-Item -Force $BkStaged $BkTarget

    $HomeDir = $env:USERPROFILE
    $ProvidersDir = Join-Path $HomeDir ".claude-bk"

    if (-not (Test-Path $ProvidersDir)) {
        New-Item -ItemType Directory -Path $ProvidersDir -Force | Out-Null
        Write-Host ""
        Write-Host "Created providers directory: $ProvidersDir"
        Write-Host "Add a provider by placing a config file at: $ProvidersDir\<name>.json"
    }

    Write-Host ""
    Write-Host "Installed to: $BkTarget"
    Write-Host "Test it with: cd <your-project>; claude-bk <provider-name> push"
}
catch {
    Write-Error "Error: $_"
    exit 1
}
finally {
    Remove-Item -Force $TmpBin -ErrorAction SilentlyContinue
    if ($BkStaged -and (Test-Path $BkStaged)) { Remove-Item -Force $BkStaged -ErrorAction SilentlyContinue }
}
