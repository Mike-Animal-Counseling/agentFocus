# AgentFocus — Future TODO & Roadmap

> 记录未来可能的功能 / UX 拓展。分三档：**用户已提出**、**作者建议（开发过程中发现的价值点）**、**技术债 / 加固**。
> 每条尽量标注：动机、实现思路、已知难点、依赖。

---

## A. 用户已明确提出的优化

### A1. 智能前台检测 —— 避免重复调起已在第一层的窗口
**动机**：当用户已经**主动**切到 VSCode 或放松浏览器时，当前页面已经是前台/第一层了，AgentFocus 不应再重复调用 `SetForegroundWindow` / AttachThreadInput 去"抢"前台——那是多余动作，可能造成闪烁或打断用户。但后续 workflow（开始放松 / 跳回 IDE / 授权弹窗）仍要正常保持。

**实现思路**：
- 在 `raiseWindowByPID` / `bringToForeground` 执行前，先用 `GetForegroundWindow()` 查当前前台窗口。
- 如果**目标窗口已经是前台窗口**（hwnd 相同，或属于同一进程 PID）→ 直接 `return`，跳过整个抢前台流程。
- 关键：判断"谁是第一层"要动态、实时——不能缓存，因为用户随时手动切。
- 保留现有的"被最小化才恢复"逻辑（`unminimizeOnly`），这俩正交。

**已知难点**：
- "同一进程"判断对 Chrome 要小心：放松窗口和用户日常 Chrome 是不同 user-data-dir，但进程名都是 chrome.exe。已有的 PID 追踪（relax-profile 专属进程）能区分，复用它即可。
- 多显示器 / 虚拟桌面下"前台"的语义。

**依赖**：纯 Win32，无外部依赖。**复杂度：低**。这是最该先做的一条。

---

### A2. 授权命令的 AI 解释 + 风险评分（放松界面内提示）
**动机**：用户在放松（刷抖音/小红书）时，Codex 弹出命令执行授权。用户脱离了上下文，光看一条 `curl ...` / `rm ...` 不知道该不该批。希望：
1. 用一个**小模型（small LLM）** 结合**最近的对话上下文**，把"Codex 为什么要跑这条命令"总结成一句人话 explanation，显示给正在放松的用户。
2. 基于一定**规则**给命令评一个 **risk score**；规则由 LLM 判断或绑定到 root/权限等级（高危操作如删文件、提权、访问敏感路径分数高）。
3. 让用户在放松界面（或授权弹窗里）一眼看懂 + 看到风险等级，再决定 Allow/Deny/Skip。

**实现思路（分层，可渐进）**：
- **数据来源**：hook payload 已经带 `tool_input.command`；上下文可从 `transcript_path`（payload 里有，指向 `~/.codex/sessions/.../*.jsonl`）读最近几轮对话。
- **风险评分 v1（规则，无 LLM，先做）**：本地正则/关键词规则给基础分——
  - 高危：`rm -rf`、`del`、`format`、写 `C:\Windows` / 系统路径、`runas` / 提权、`Invoke-Expression`、下载并执行（`curl ... | sh`）、改 registry。
  - 中危：网络访问、写工作区外、安装包。
  - 低危：读操作、工作区内文件、`echo` / `ls`。
  - 输出 0–100 分 + 等级（低/中/高），在 TaskDialog 里用颜色标注（绿/黄/红）。
- **AI explanation v2（接 LLM）**：把 command + 最近上下文喂给一个小模型（本地 Ollama / 云端 haiku 级），返回一句中文解释 + 可选的 risk 复核。
  - 同步调用会拖慢授权（用户在等弹窗）——需权衡：要么超时回退到纯规则分，要么先弹窗显示规则分、AI 解释异步补充刷新。
- **UI 落点**：扩展现有 TaskDialog（approval_windows.go）——把 explanation 放 content 区，risk score 放标题或用图标/颜色；或在放松窗口里浮一个角标。

**已知难点 / 岔路**：
- **延迟**：授权是同步阻塞流程（`/approval` 端点等用户点）。LLM 调用几百 ms~几秒，会让弹窗变慢。建议：规则分**立即**出，LLM 解释**异步**补。
- **小模型部署**：本地（Ollama，隐私好、要装）vs 云端（haiku，要 API key、命令内容出网——敏感）。需让用户选 + 默认关。
- **"rule determined by LLM or root auth level"**：这句有两种解读——(a) 规则本身由 LLM 生成；(b) 风险阈值随用户权限等级变。建议先做**静态规则表**，LLM 只做解释；规则可配置化留到后面。
- **隐私**：把命令 + 对话上下文发给云端 LLM 是隐私敏感操作，必须显式 opt-in。

**依赖**：LLM 运行时（Ollama 或 API）、读 session jsonl 的 parser。**复杂度：中高**。建议拆成「v1 纯规则评分」先落地，「v2 AI 解释」后接。

---

## B. 作者建议的拓展（开发过程中发现的价值点）

### B1. 配置可视化 / 托盘内开关
现在改 config 要手动编辑 `%APPDATA%\AgentFocus\config.json` 再重启。可在托盘菜单加子项直接切：放松开关、弹窗开关、放松网址增删。`config.go` 已有 `RelaxEnabled`/`PopupEnabled` 字段，但**没有热重载**——改完需重启进程才生效。可加文件监听（fsnotify）热重载。

### B2. 放松内容可配置 + 轮换
`RelaxURLs` 现在写死抖音+小红书。可让用户在配置里自定义；甚至按时段/心情轮换不同网站（B站、YouTube、音乐）。

### B3. 专注 / 摸鱼数据统计
记录每天 Codex 跑了多少 turn、放松了多久、批了/拒了多少授权。托盘里看「今日专注 2h / 放松 25min」。数据已经天然流经 hook（UserPromptSubmit/Stop 带时间戳），落个本地 sqlite 即可。

### B4. 倒计时窗口可交互
当前倒计时窗口是只读 toast。可加按钮：「立即跳回」「再等 30 秒」「这次别打扰」，给用户对"何时被拉回工作"的控制权。

### B5. 多 IDE 支持
现在 `ideWindowTitle = "Visual Studio Code"` 写死。可支持 Cursor、JetBrains、Windsurf 等——配置化窗口标题匹配，或自动探测 Codex 是从哪个 IDE 拉起的。

### B6. 放松时的"软提醒"而非硬跳转
有些用户可能不喜欢被强制拉回 VSCode。可选模式：完成时只发个通知（声音/角标），让用户自己决定何时回去，而不是 3 秒后强制 SetForegroundWindow。

### B7. 安装 / 分发
现在是手动 `go build` + 双击 exe + 手动配 hook。可做：
- 一键安装脚本（写 config.toml hook、设开机自启、放快捷方式）。
- 开机自启选项（之前讨论过，未做）。
- 签名 exe，避免 SmartScreen 拦截。

---

## C. 技术债 / 加固

### C1. 备用实现的去留
`watcher/codex.go`（app-server 旧路）、`watcher/fake.go`、`actuator/fake.go` 当前**有定义无调用**，保留作备用/参考。若长期不用可删；若保留，建议加注释标注"备用，未接线"。

### C2. UI 测试覆盖
`internal/ui/` 全是 Win32 调用，目前**无单元测试**（难测）。可考虑：把可测的纯逻辑（决定映射、风险评分规则）抽离出来单测；Win32 部分靠手动/脚本冒烟。

### C3. hook 信任 hash 漂移
每次改 `hook_probe.ps1` 的命令行（config.toml 里），hash 变 → Codex 要求重新 Trust。开发期频繁触发。可考虑：脚本逻辑稳定后固定命令行，或文档化"改脚本后需重新 Trust"。

### C4. 错误可观测性
正式版 `-H windowsgui` 无控制台，出问题看不到日志。已有 `hook_received.log`，可扩展成一个统一的轮转日志文件（`%APPDATA%\AgentFocus\agentfocus.log`），方便用户/作者排查。`AgentFocus-debug.exe`（带控制台）作为现场排查工具保留。

### C5. 并发授权的 UI 排队体验
`/approval` 已能并发接收、UI Manager 串行弹窗。但多个授权同时来时，用户要逐个点，体验可优化（比如显示"还有 N 个待批"、或批量 Allow）。

### C6. 端口冲突处理
HTTP server 固定 `27182`。若被占用，当前启动会失败但无明显提示。可加端口探测 + 回退 + 在托盘提示。

---

## 优先级建议（作者视角）
1. **A1 智能前台检测** — 低成本、立刻改善体验、纯本地。
2. **A2-v1 规则风险评分** — 中等成本、安全价值高、不依赖 LLM。
3. **B3 数据统计 / B1 托盘开关** — 提升日常可用性。
4. **A2-v2 AI 解释** — 高价值但有延迟/隐私/部署难点，需设计清楚再做。
5. 其余按需。
