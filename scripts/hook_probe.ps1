# AgentFocus hook forwarder.
#
# Codex invokes this for UserPromptSubmit / PermissionRequest / Stop, passing a
# UTF-8 JSON payload on stdin.
#
#   - PermissionRequest: POST to /approval and BLOCK until AgentFocus returns the
#     user's decision, then emit that decision on stdout in the format Codex
#     expects (hookSpecificOutput). Codex reads stdout to allow/deny without
#     prompting. "skip" -> emit nothing, so Codex falls back to its own prompt.
#   - Other events: POST to /hook fire-and-forget (don't block Codex).
#
# Encoding: read and forward raw UTF-8 BYTES. On zh-CN Windows the default is
# GBK; decoding then re-encoding corrupts the Chinese `description` and breaks the
# JSON. We only decode a copy to read the ASCII event name.
#
# Must never fail or hang Codex unexpectedly: errors are swallowed and we exit 0.
# (The long block on /approval is intentional; timeout is set high in config.toml.)

$ErrorActionPreference = 'SilentlyContinue'

$bytes = $null
try {
    $stdin = [Console]::OpenStandardInput()
    $ms = New-Object System.IO.MemoryStream
    $stdin.CopyTo($ms)
    $bytes = $ms.ToArray()
} catch {}

if (-not $bytes -or $bytes.Length -eq 0) { exit 0 }

$text = [System.Text.Encoding]::UTF8.GetString($bytes)

if ($text -match '"hook_event_name"\s*:\s*"PermissionRequest"') {
    # Synchronous: ask AgentFocus, block for the user's decision.
    try {
        $resp = Invoke-WebRequest `
            -Uri 'http://localhost:27182/approval' `
            -Method Post `
            -ContentType 'application/json; charset=utf-8' `
            -Body $bytes `
            -UseBasicParsing
        $decision = (ConvertFrom-Json $resp.Content).decision
    } catch {
        # AgentFocus unreachable: skip (let Codex decide).
        $decision = 'skip'
    }

    if ($decision -eq 'allow' -or $decision -eq 'deny') {
        $out = '{"hookSpecificOutput":{"hookEventName":"PermissionRequest","decision":{"behavior":"' + $decision + '"}}}'
        [Console]::Out.Write($out)
        [Console]::Out.Flush()
    }
    # skip -> emit nothing; Codex uses its normal approval flow.
    exit 0
}

# Non-PermissionRequest: fire-and-forget to /hook.
try {
    Invoke-WebRequest `
        -Uri 'http://localhost:27182/hook' `
        -Method Post `
        -ContentType 'application/json; charset=utf-8' `
        -Body $bytes `
        -TimeoutSec 2 `
        -UseBasicParsing | Out-Null
} catch {}

exit 0
