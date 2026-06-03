# AgentFocus installer (Windows).
#
# Configures the Codex hooks that AgentFocus relies on. Run this once after
# cloning + building. It:
#   1. Locates this repo and the hook forwarder script (scripts/hook_probe.ps1).
#   2. Adds/updates the UserPromptSubmit / PermissionRequest / Stop hooks in
#      %USERPROFILE%\.codex\config.toml, pointing at this repo's hook script.
#   3. Leaves your existing config.toml settings untouched (only manages the
#      AgentFocus hook block, delimited by marker comments).
#
# After running: start AgentFocus.exe, then run `codex` once and choose
# "Trust all" when it asks to review the new hooks.
#
# Re-running is safe: it replaces the previous AgentFocus hook block.

$ErrorActionPreference = 'Stop'

# --- locate paths ---------------------------------------------------------
$repoRoot = Split-Path -Parent $PSScriptRoot           # repo root (scripts/..)
$hookScript = Join-Path $PSScriptRoot 'hook_probe.ps1' # this repo's forwarder
if (-not (Test-Path $hookScript)) {
    throw "hook_probe.ps1 not found at $hookScript"
}

$codexHome = Join-Path $env:USERPROFILE '.codex'
$configToml = Join-Path $codexHome 'config.toml'
if (-not (Test-Path $codexHome)) {
    throw "Codex home not found at $codexHome. Install Codex CLI first."
}

Write-Host "AgentFocus installer"
Write-Host "  repo:        $repoRoot"
Write-Host "  hook script: $hookScript"
Write-Host "  config.toml: $configToml"
Write-Host ""

# --- build the hook block -------------------------------------------------
# Each hook runs the forwarder hidden. PermissionRequest uses a high timeout
# because it blocks waiting for the user's Allow/Deny/Skip decision.
$cmd = "powershell -WindowStyle Hidden -NoProfile -ExecutionPolicy Bypass -File `"$hookScript`""

$beginMarker = '# === AgentFocus hooks (managed by install.ps1) ==='
$endMarker   = '# === end AgentFocus hooks ==='

$block = @"
$beginMarker
# Forwards UserPromptSubmit / PermissionRequest / Stop to AgentFocus
# (http://localhost:27182). Do not edit by hand; re-run install.ps1 instead.

[[hooks.UserPromptSubmit]]
matcher = ".*"
[[hooks.UserPromptSubmit.hooks]]
type = "command"
command = "true"
command_windows = '$cmd'
timeout = 30
statusMessage = "AgentFocus"

[[hooks.PermissionRequest]]
matcher = ".*"
[[hooks.PermissionRequest.hooks]]
type = "command"
command = "true"
command_windows = '$cmd'
timeout = 86400
statusMessage = "AgentFocus"

[[hooks.Stop]]
matcher = ".*"
[[hooks.Stop.hooks]]
type = "command"
command = "true"
command_windows = '$cmd'
timeout = 30
statusMessage = "AgentFocus"
$endMarker
"@

# --- merge into config.toml ----------------------------------------------
$existing = ""
if (Test-Path $configToml) {
    $existing = Get-Content $configToml -Raw -Encoding UTF8
}

# Remove any previous AgentFocus block (between markers, inclusive).
if ($existing -match [regex]::Escape($beginMarker)) {
    $pattern = "(?s)" + [regex]::Escape($beginMarker) + ".*?" + [regex]::Escape($endMarker) + "\r?\n?"
    $existing = [regex]::Replace($existing, $pattern, "")
    Write-Host "Replaced existing AgentFocus hook block."
}

# Append the fresh block.
$existing = $existing.TrimEnd() + "`r`n`r`n" + $block + "`r`n"

# Back up then write.
if (Test-Path $configToml) {
    Copy-Item $configToml "$configToml.agentfocus.bak" -Force
    Write-Host "Backed up config.toml -> config.toml.agentfocus.bak"
}
Set-Content -Path $configToml -Value $existing -Encoding UTF8 -NoNewline

Write-Host ""
Write-Host "Done. Next steps:" -ForegroundColor Green
Write-Host "  1. Start AgentFocus.exe (a tray icon appears)."
Write-Host "  2. Run 'codex' once; when it says 'hooks need review', choose 'Trust all'."
Write-Host "  3. Use Codex as usual — AgentFocus reacts to its hooks."
