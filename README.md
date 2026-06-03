# AgentFocus

**Relax while Codex works — without losing control.**

AgentFocus is a lightweight Windows tray app that reacts to [OpenAI Codex](https://developers.openai.com/codex)'s activity. When you send Codex a prompt, it opens a "relax" browser window so you can take a break; when Codex needs your approval to run a command, it shows a topmost dialog so you decide **Allow / Deny / Skip** without hunting for the terminal; when Codex finishes, it counts down and brings your IDE back to the foreground.

It hooks into Codex via Codex's own **hooks** mechanism (configured in `~/.codex/config.toml`), so it works alongside the Codex CLI and the VSCode extension without changing your workflow.

---

## How it works

```
 Codex (CLI or VSCode ext)
      │  hooks (config.toml)
      ▼
 hook_probe.ps1  ──HTTP──▶  AgentFocus.exe  (localhost:27182)
      │                         │
      │                         ├─ UserPromptSubmit → open relax browser
      │                         ├─ PermissionRequest → Allow/Deny/Skip dialog ──┐
      │  ◀── decision (stdout) ─┘  (Codex honors the returned decision)        │
      │                         └─ Stop → countdown → bring IDE to foreground   │
      └──────────────────────────────────────────────────────────────────────┘
```

- **`/hook`** — fire-and-forget events (`UserPromptSubmit`, `Stop`) drive the relax/restore behavior.
- **`/approval`** — `PermissionRequest` blocks synchronously until you pick Allow/Deny/Skip; the decision is fed back to Codex via the hook's stdout, so Codex runs or rejects the command accordingly.

The relax browser runs in a dedicated Chrome profile (`%LOCALAPPDATA%\AgentFocus\relax-profile`) so it's isolated from your normal browsing and its login state persists.

---

## Requirements

- **Windows 10/11**
- **[Codex CLI](https://developers.openai.com/codex)** installed and logged in (the VSCode Codex extension also works — it shares `~/.codex/config.toml`)
- **[Go](https://go.dev/dl/) 1.24+** (only to build from source)
- **Google Chrome** (for the relax window; falls back to the default browser if Chrome isn't found)

---

## Install

### Option A — download a release
1. Grab `AgentFocus.exe` from the [Releases](../../releases) page.
2. Clone or download this repo (you need `scripts/hook_probe.ps1` and `scripts/install.ps1`).
3. Run the installer (configures the Codex hooks):
   ```powershell
   powershell -ExecutionPolicy Bypass -File scripts\install.ps1
   ```
4. Double-click `AgentFocus.exe` (a tray icon appears).
5. Run `codex` once; when it says **"hooks need review"**, choose **Trust all and continue**.

### Option B — build from source
```powershell
git clone https://github.com/Mike-Animal-Counseling/agentFocus.git
cd agentFocus

# Build (embeds the icon + the comctl32-v6 manifest the approval dialog needs)
go build -ldflags="-H windowsgui" -o AgentFocus.exe ./cmd/agentfocus

# Configure Codex hooks
powershell -ExecutionPolicy Bypass -File scripts\install.ps1

# Run
.\AgentFocus.exe
```

Then run `codex` once and **Trust all** when prompted.

> The first time the relax window opens, the dedicated Chrome profile is empty — log in to your relax sites once and it'll remember you afterward.

---

## Usage

Once installed and the hooks are trusted, just use Codex normally:

| You do | AgentFocus does |
|---|---|
| Send a prompt | Opens / focuses the relax browser window (maximized first time, keeps your size after) |
| Codex asks to run a command | Shows a topmost **Allow / Deny / Skip** dialog with the command and Codex waits for your choice |
| Codex finishes the turn | Shows a bottom-right countdown, then brings your IDE back to the foreground |

**Tray menu:** pause/resume AgentFocus, open the config file, quit.

---

## Configuration

Settings live in `%APPDATA%\AgentFocus\config.json` (created on first run):

| Key | Default | Meaning |
|---|---|---|
| `relaxURLs` | douyin + xiaohongshu | URLs opened in the relax window |
| `relaxEnabled` | `true` | Open the relax browser at all |
| `popupEnabled` | `true` | Show the approval dialog |
| `hookServerPort` | `27182` | Local HTTP port hooks POST to |

Edit it (the tray's "open config file" opens it for you), then restart AgentFocus.

---

## Project layout

```
cmd/agentfocus/      entry point + assembly; app icon/manifest resource (.syso)
internal/event/      event & action types (pure data)
internal/watcher/    HTTP hook server (httpserver.go); codex.go/fake.go are spare
internal/core/       state machine: events → actions
internal/actuator/   browser (relax), ide (foreground), dispatcher
internal/ui/         tray, approval dialog (TaskDialog), countdown toast
internal/config/     config struct + JSON load/save
scripts/             hook_probe.ps1 (runtime forwarder), install.ps1
```

See [FUTURE_TODO.md](FUTURE_TODO.md) for planned features and ideas.

---

## Build notes

- The build embeds `cmd/agentfocus/rsrc_windows_amd64.syso`, which bundles the **app icon** and a **manifest declaring comctl32 v6** — the approval dialog (`TaskDialogIndirect`) requires v6, so don't remove the syso/manifest.
- To regenerate the icon: `go run _probe/genicon/main.go internal/ui/icon.ico`, then rebuild the syso with [go-winres](https://github.com/tc-hib/go-winres).
- `go build ./...`, `go vet ./...`, and `go test ./...` should all pass.

---

## License

MIT (see LICENSE).
