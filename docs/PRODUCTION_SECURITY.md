# Production security requirements

AgentFocus is a per-user Windows desktop application, not a public HTTP service. Keep the hook listener bound to 127.0.0.1. A new 256-bit hook token is generated on startup under `%APPDATA%/AgentFocus/auth/hook.token`; Windows ACLs restrict the directory/file to that user and SYSTEM. The updated `scripts/hook_probe.ps1` reads the token and forwards it as a Bearer header. Do not publish the token or expose/forward port 27182.

The listener rejects unrecognized Host values, browser Origin headers, unauthenticated callers, non-JSON content and bodies over 64 KiB. Active requests and server timeouts are bounded. Hook payloads are no longer written to debug logs. Existing `hook_received.log` files created by older builds can contain private commands and prompts; remove them through your normal private-data retention procedure.

Run Go 1.27.2 or a newer patched supported release. CI runs tests, vet and govulncheck. Default-browser URLs are restricted to HTTP(S) and launched using the Windows Shell API without shell command concatenation. The token does not protect against a process already running as the same Windows user; use OS account isolation for that boundary.
