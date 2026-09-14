# 建立 `codex-village`

建立 **codex-village**：一個迷你的即時島嶼／小鎮／RTS 棋盤／立體模型，用來即時視覺化 **Codex root agent 與其 subagents**。

它的核心目標，是把 Codex Multi-Agent 的執行過程轉成一個空間化世界。

例如：

```text
root Codex thread
      │
      ├── subagent A
      ├── subagent B
      │      └── subagent C
      └── subagent D
```

Root Codex agent 是 orchestrator。

每一個被建立的 Codex subagent，都要變成地圖上一個可見的 villager／unit。

它們的：

* 狀態
* tool activity
* delegation
* waiting
* completion
* interruption
* error

都應該轉換成：

* 移動
* 動畫
* 特效
* 空間關係

不要 clone 其他視覺化 repo。

如果目前 repo 已經是這個專案，就直接修改。

如果還沒有 repo，就建立新的 `codex-village` repository。

---

# 0. 視覺風格 Look

**在畫任何 map、sprite、角色、建築、unit、HUD skin 或 player character 之前，先詢問使用者想要什麼視覺風格。**

不要替使用者決定。

不要偷偷使用預設風格。

可以提供一些方向幫助使用者思考，例如：

* 可愛 PS1 低多邊形小鎮
* SNES JRPG 村莊
* Game Boy 黑白風
* Stardew-like pixel town
* Warcraft III 類 RTS 基地
* 迷你軟體工程師辦公室
* Cyberpunk command center
* 紙藝立體模型
* 微縮火車模型小鎮
* 綠色 phosphor terminal 世界
* 其他風格

這些只是參考。

如果使用者提供圖片，就跟著參考圖。

如果使用者文字描述風格，就依照描述。

如果使用者說：

> 都可以

或沒有真正選擇，不要直接開始畫。

再：

* 問一個更具體的問題，或
* 提出 2～3 個差異明顯的方向

**直到使用者真正選定風格後，才能開始繪製。**

選定的視覺風格會影響：

* map
* sprites
* buildings
* workstations
* nametags
* speech bubbles
* tool overlays
* idle animations
* synthesized sound effects
* root-agent HQ
* subagent homes / work areas
* player 外觀

但不得改變底層 Codex integration architecture。

---

# 1. 核心心智模型

不要把 Codex agents 當成平面的 bot roster。

它們應該是一棵 **execution tree**。

```text
                    ROOT AGENT
                   orchestrator
                        │
             ┌──────────┼──────────┐
             │          │          │
             ▼          ▼          ▼
          explorer    worker     worker
             │
             ▼
          worker
```

每一個視覺單位都對應一個真正的 Codex thread / subagent。

必須明確區分：

```text
root thread
subagent thread
player
```

其中：

**player 代表人類使用者。**

Player 永遠不是 Codex thread。

---

# 2. Codex 整合架構

優先使用已安裝的 **Codex App Server** 作為 structured control plane。

啟動：

```bash
codex app-server
```

在 managed mode 下，將它作為長時間執行的 child process。

Go server 與 Codex App Server 之間透過：

```text
stdin/stdout
```

進行：

```text
JSONL framed JSON-RPC
```

通訊。

開發或啟動時，先檢查目前安裝版本的 protocol。

不要直接 hard-code 對舊版 Codex protocol 的假設。

執行：

```bash
codex app-server generate-json-schema --out ./generated/codex-schema
```

以目前本機安裝版本的 schema 作為 protocol source of truth。

不要自行維護會隨 Codex 更新而改變的 enum 複本。

重要 protocol concepts 包含：

```text
initialize
initialized

thread/start
thread/read
thread/list
thread/resume

turn/start
turn/steer
turn/interrupt

thread/started
turn/started
turn/completed
turn/failed
turn/cancelled

item/*
codex/event/*
```

Codex protocol 未來可能增加 event。

對未知 event：

```text
安全忽略
不要 crash
不要把 raw payload 顯示到前端
```

---

# 3. 兩種執行模式

支援兩種模式。

---

## Observer mode

用途：

在不接管 Codex session 的情況下，視覺化目前既有的 Codex CLI / Desktop sessions。

從：

```text
$CODEX_HOME
```

讀取資料。

預設：

```text
~/.codex
```

Codex canonical local sessions 通常存在：

```text
~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl
```

這些 JSONL 檔案全部視為：

```text
read-only
```

永遠不要修改它們。

Observer mode 主要架構：

```text
Codex
   ↓
rollout JSONL
   ↓
codex-village
   ↓
Canvas visualization
```

使用：

* filesystem notification
* 輕量 poll fallback

追蹤檔案變化。

---

## Managed mode

用途：

讓 `codex-village` 自己 host / control 一個 Codex session。

架構：

```text
Browser
   │
   ▼
Go server
   │
   ▼
codex app-server
   │
   ▼
Codex root thread
   │
   ├── subagent
   ├── subagent
   └── subagent
```

Go server 擁有 Codex App Server child process 與其：

```text
stdin
stdout
```

Browser 絕對不能直接與 Codex App Server 通訊。

Managed mode 使用 structured App Server notifications。

只要 App Server event 可用，就優先使用它，而不是 filesystem heuristic。

---

# 4. Session 選擇

Codex 可能有大量歷史 session。

不要把所有歷史 session 都畫成 villager。

一次只視覺化：

**一棵 root execution tree。**

支援：

```text
--thread <thread-id>
--latest
--cwd <path>
```

如果沒有指定，顯示一個最小化 session picker，讓使用者選擇 root session。

一旦 root thread 選定，只發現：

```text
root thread
+
它真正 spawn 出來的 descendants
```

不要把沒有關聯的 Codex session 混進同一個世界。

---

# 5. 判斷 root 與 subagent

只要 Codex 有 structured metadata，就優先使用。

Codex Multi-Agent 建立的 child thread 必須被辨識為 subagent。

如果存在：

```text
ThreadSpawn
SubAgent
parent thread metadata
```

等資訊，就用它建立真正的 parent-child 關係。

內部維護 graph：

```go
type AgentNode struct {
    ThreadID       string
    ParentThreadID string
    AgentType      string
    Model          string
    Title          string
    Task           string
    Status         string
    ActiveTurnID   string
    LastActivity   time.Time
}
```

Hierarchy 很重要。

Child agent 必須連到它真正的 parent。

不要因為兩個 session 建立時間很接近，就猜它們是 parent-child。

---

# 6. 世界表示方式

選定的 root thread 代表這個世界的主要控制中心。

依照風格，可以視覺化為：

* town hall
* command center
* guild hall
* castle
* lab
* HQ
* mothership
* main terminal

Root agent 不一定要被畫成普通 villager。

Root 應該清楚看起來像：

```text
orchestrator
```

Subagents 則是：

```text
villagers
units
workers
```

---

# 7. Subagent lifecycle → 動畫

把 Codex 真實 lifecycle state 對應成地圖行為。

---

## Spawn

當 root agent 建立 subagent：

```text
spawn_agent
        ↓
new child thread
        ↓
unit enters world
```

新角色可以從以下位置出現：

* HQ
* spawn gate
* tent
* workstation
* train station
* portal

依照目前選定的視覺風格決定。

---

## pendingInit

視覺狀態：

```text
getting ready
```

例如：

* 伸懶腰
* 打開 laptop
* 戴上 helmet
* 啟動 workstation

---

## running

Agent 走向工作區。

不同 activity 可以對應不同地方。

例如：

```text
source inspection  → bookshelf / terminal
coding             → desk / forge
testing            → test bench
web research       → telescope / radio
git operations     → depot
review             → inspection table
```

不要所有 tool activity 都只使用同一種動畫。

---

## reasoning

使用低調的 thinking animation。

例如：

```text
...
?
small gears
thought cloud
scroll
terminal glow
```

但是：

**絕對不能顯示 private chain-of-thought。**

只能視覺化：

> agent 現在正在 reasoning

不能顯示 reasoning text 本身。

---

## Tool call

顯示短暫 generic work overlay。

例如：

```text
hammer
wrench
terminal spark
magnifying glass
folder
test flask
git branch
```

不要在地圖 bubble 中顯示：

* shell command
* tool arguments
* tokens
* secrets
* 含敏感路徑的完整 filename

---

## 等待 input / approval

這個狀態必須非常明顯。

例如：

```text
!
raised hand
doorbell
flashing terminal
question bubble
```

它代表：

> 這個 agent 現在需要人類注意。

視覺優先級要比普通 idle 更高。

---

## completed

Agent 完成任務時要有清楚的結束動畫。

例如：

```text
check mark
small celebration
pack tools
walk away from workstation
```

完成後仍留在世界裡。

不要完成後立刻刪除 subagent。

---

## interrupted

使用明確 stop tell。

例如：

```text
red stop sign
dropped tool
pause icon
```

---

## errored

使用明顯但不過度誇張的錯誤狀態。

例如：

```text
smoke puff
broken wrench
warning triangle
```

錯誤的 agent 仍要留在畫面。

---

## 長時間 quiet

沒有活動的 agent 仍然可見。

可以：

* 坐著
* 喝東西
* 釣魚
* 靠著牆
* 打瞌睡

長時間沒活動可以顯示：

```text
Zzz
```

不要因為 idle 就把 agent 藏到建築物裡。

---

# 8. Delegation 必須被視覺化

這是 Codex 版本最重要的功能之一。

如果 parent agent spawn 一個 child：

```text
root
  │
  └──── assignment scroll ────> worker A
```

如果 `worker A` 又建立 `worker B`：

```text
root
  │
  ▼
worker A
  │
  └──── assignment ────> worker B
```

使用者應該只看地圖就能理解：

> 誰把工作交給誰？

可以用：

* scroll
* letter
* beam
* signal line
* dispatch animation

呈現 delegation。

不要顯示完整 delegated prompt。

只顯示簡短 sanitized task label。

例如：

```text
tests
search
review
frontend
debug
docs
```

---

# 9. Agent 命名

如果 Codex 有提供 nickname / name，就使用它。

如果沒有，就產生 stable short display label。

例如：

```text
explorer-1
worker-2
reviewer
subagent-a3f2
```

不要把完整 UUID 當成一般 nametag。

完整 thread ID 只保存在 backend。

Nametag：

```text
bold 7px monospace
```

畫在 rounded pill 上。

直接畫到低解析度 Canvas。

---

# 10. Agent card

點擊 subagent 時打開一張：

**slim fixed overlay card**

不能造成 map layout shift。

使用：

```css
position: fixed;
```

Card 顯示：

```text
avatar
display name
agent type
model（如果能可靠取得）
short task label
state
last activity kind
parent agent
```

範例：

```text
┌────────────────────────┐
│ 🐸  explorer-1         │
│ Explorer               │
│ gpt-5.x                │
│ task: search           │
│ ● running              │
│ last: tool             │
│ parent: root           │
└────────────────────────┘
```

不要顯示：

```text
raw prompts
raw tool args
transcript bytes
JSONL line counts
events/minute
token dumps
完整 transcript snippets
hidden reasoning
```

---

# 11. Avatar

Route：

```text
/avatars/<thread-id>
```

如果 Codex 沒有提供 avatar，就依 thread ID deterministic 生成。

相同 thread ID：

```text
永遠產生同一張 avatar
```

使用：

* 小 palette
* 簡單 facial geometry

並符合目前選定的美術風格。

Root agent 要有和一般 subagent 明顯不同的：

* avatar
* badge
* icon

---

# 12. Player character

人類使用者也存在地圖上。

Nametag：

```text
you
```

Player：

* 不是 agent
* 不是 Codex thread
* 不屬於 agent roster
* 沒有 house
* 不算 worker

預設待在：

```text
地圖中心附近 / root HQ 附近
```

不要加入：

* WASD
* 手動方向控制

Player 只因互動而自動移動。

在畫 player 前，先詢問使用者外觀。

提供 2～3 個符合當前世界風格的簡單提案。

不要：

* 複製名人
* 模仿受版權保護的角色

---

# 13. 從世界對 agent 發 Prompt

Prompt bar 是：

```text
single-line HUD overlay
```

位置：

```text
地圖下方中央
```

尚未選取 agent：

```text
click an agent first…
```

選取後：

```text
message <agent-name>…
```

不要把 prompt input 放在 agent card 裡。

使用：

```css
position: fixed;
```

Focus：

```js
input.focus({ preventScroll: true })
```

地圖不能因 input focus 而移動。

---

# 14. Prompt routing

Browser 呼叫：

```http
POST /api/prompt
Content-Type: application/json
```

Body：

```json
{
  "threadId": "...",
  "prompt": "..."
}
```

Browser 絕對不能直接與 Codex 通訊。

由 Go server 決定要如何把 prompt 送入 Codex。

---

# 15. Managed mode message behavior

當 `codex-village` 自己擁有 App Server session 時：

---

## 如果目標 thread 已經有 active ordinary turn

使用：

```text
turn/steer
```

傳入：

```text
threadId
expectedTurnId
input
```

不要猜 `expectedTurnId`。

必須使用實際從 Codex event 得到的 active turn ID。

---

## 如果目標 thread 是 idle

使用：

```text
turn/start
```

在同一個 thread 啟動新的 turn。

不要為了傳訊息給既有 agent，而另外建立新的 thread。

---

## Child thread 相容性

Codex Multi-Agent protocol 未來可能改變。

在允許直接 prompt child agent 前，先確認目前 runtime / App Server 是否支援該操作。

如果目前 Codex 版本拒絕 direct child input：

* 顯示清楚 status
* 保留世界狀態
* 不要偷偷 reroute 到其他 thread
* 不要假裝送出成功

例如：

```text
this Codex version won't accept direct child input
```

---

# 16. Observer mode prompting 安全

Observer mode 可能正在監看另一個：

* Codex CLI
* Codex Desktop

所擁有的 session。

不要和原本 client 搶著修改同一個 session。

Observer mode 預設：

```text
visualization = enabled
direct prompt = disabled
```

除非已經建立明確的 session ownership / safe control。

如果使用者想要直接從地圖對 agent 發話，可以：

* 切到 managed mode
* 或明確透過 App Server resume thread

不要為了 UI 方便就建立 competing turns。

---

# 17. Optimistic player animation

當 prompt 被 UI 接受準備送出時：

1. 立即清空 input。
2. Status：

```text
on my way
```

3. Player 開始走向 selected subagent。
4. 如果 subagent 正在移動，追蹤目前位置。
5. 如果它視覺上正在睡覺，把它叫醒。
6. Player 抵達時，顯示 player speech bubble。
7. Bubble 顯示真實 message 的短截取。
8. Status：

```text
said it
```

9. Go server 再透過 Codex bridge 送出訊息。

如果送出失敗：

```text
failed: <short safe reason>
```

不要讓 Player 倒退回原位。

不要因失敗 undo 動畫。

不要把可能含敏感資訊的 raw JSON-RPC error 顯示到前端。

---

# 18. Speech bubble

Subagent bubble 只能顯示 generic world chatter。

例如：

```text
checking…
on it!
hmm…
testing…
one sec…
found something!
almost done…
```

不要顯示真實：

```text
source code
prompt
tool command
search query
tool arguments
reasoning
file content
```

Player bubble 可以顯示：

> 人類剛傳送的 message 的短截取

保留原始大小寫。

長度約：

```text
22 characters
```

只顯示一行。

---

# 19. Backend normalized event model

Frontend renderer 不應該直接理解 Codex raw JSON。

所有 Codex events 都先在 Go server 做 normalization。

例如：

```go
type Activity struct {
    Type       string    `json:"type"`
    ThreadID   string    `json:"threadId"`
    ParentID   string    `json:"parentId,omitempty"`
    Name       string    `json:"name"`
    AgentType  string    `json:"agentType,omitempty"`
    State      string    `json:"state"`
    Activity   string    `json:"activity"`
    Goal       string    `json:"goal,omitempty"`
    ToolKind   string    `json:"toolKind,omitempty"`
    Timestamp  time.Time `json:"ts"`
}
```

Frontend 接收到的應該是穩定格式。

例如：

```json
{
  "type": "activity",
  "threadId": "abc",
  "parentId": "root",
  "name": "explorer-1",
  "state": "running",
  "activity": "tool",
  "goal": "search",
  "ts": "..."
}
```

Canvas renderer 不應依賴 Codex App Server raw schema。

---

# 20. Burst coalescing

Codex streaming 可能在短時間產生大量 events。

不要每一個 delta 都做一次動畫。

將約：

```text
200–300 ms
```

內的 burst 合併成一個 semantic activity。

例如：

```text
50 command/output deltas
        ↓
one "tool working" animation
```

以及：

```text
many reasoning deltas
        ↓
one thinking state
```

這是維持：

```text
60fps
```

的重要條件。

---

# 21. 敏感資料邊界

Codex transcript 與 tool payload 都應視為 sensitive。

永遠不要把 raw rollout JSONL 傳到 browser。

永遠不要把 hidden reasoning 傳到 browser。

不要 render：

```text
API keys
cookies
authorization headers
environment secrets
完整 shell commands
private source files
完整 prompts
tool responses
```

未來可以另外做 secure transcript inspector。

但：

**這不屬於 MVP。**

目前 `codex-village` 是：

```text
state visualization
```

不是：

```text
transcript viewer
```

---

# 22. Go Stack

使用：

```text
Go 1.24+
```

搭配：

```go
//go:embed static
```

不要 Node build step。

不要 bundler。

Frontend：

```text
vanilla HTML
vanilla CSS
vanilla JavaScript
Canvas 2D
```

Routes：

```text
/                       SPA
/ws                     normalized live events
/api/threads            current execution tree
/api/thread/<id>        slim metadata
/api/prompt             prompt / steer target
/avatars/<thread-id>    deterministic avatar
/api/health             health
```

Listen：

```text
0.0.0.0:8040
```

不要只 listen localhost。

---

# 23. Rendering

Internal render resolution：

```text
480×270
```

放大使用 nearest-neighbor。

需要時使用：

```css
image-rendering: pixelated;
```

目標：

```text
60fps
```

支援約：

```text
30 simultaneous visible agents
```

如果選定的風格需要 perspective，可以使用：

* billboard sprites
* baked environment

不要導入大型 game engine。

---

# 24. Layout stability

Agent card、HUD、prompt 出現時：

**世界不能移動。**

設定：

```css
html,
body,
#stage {
    overflow: hidden;
}
```

Card 與 HUD：

```css
position: fixed;
```

Canvas world coordinate 永遠維持不變。

---

# 25. Day / Night

日夜依照 browser local time。

支援：

```text
?hour=22
```

用來預覽指定時間。

這只影響視覺。

不得影響 Codex 的真實執行行為。

---

# 26. Sound

所有 sound effects 都在 browser 內 synthesize。

不要：

```text
MP3
WAV
external audio assets
```

第一次使用者互動後才能啟用 audio。

鍵盤：

```text
M
```

切換 mute。

音效必須符合使用者選定的視覺風格。

除非使用者特別要求，否則不要背景音樂。

---

# 27. Demo mode

提供：

```bash
go run . --demo
```

Demo 要產生一棵真實感的 multi-agent tree。

例如：

```text
root
├── explorer-1
├── worker-1
│   └── tester-1
├── reviewer
└── docs
```

模擬：

* spawn
* reasoning
* tool use
* waiting
* nested delegation
* completion
* interruption
* error
* idle
* sleep

Renderer 不得為 demo mode 寫特別邏輯。

Demo events 必須經過與 production 完全相同的 normalized event pipeline。

打開：

```text
http://localhost:8040
```

---

# 28. Live Observer Mode

例如：

```bash
CODEX_HOME="$HOME/.codex" \
go run . --latest --listen :8040
```

或：

```bash
go run . \
  --codex-home "$HOME/.codex" \
  --thread <thread-id> \
  --listen :8040
```

只 tail：

> 當前選定 execution tree 所屬的檔案。

不要把全部歷史 Codex transcript recursive load 到 memory。

啟動時：

* 可以先重建必要 metadata
* live activity 從接近 EOF 開始追蹤

不要 replay 整段歷史動畫。

---

# 29. Managed Mode

例如：

```bash
go run . \
  --managed \
  --cwd /path/to/repo \
  --listen :8040
```

Managed 架構：

```text
              Browser
                 │
          HTTP / WebSocket
                 │
                 ▼
           codex-village
             Go server
                 │
       JSON-RPC / JSONL stdio
                 │
                 ▼
        codex app-server
                 │
                 ▼
          root Codex thread
            /      |      \
           /       |       \
      explorer   worker   worker
```

Codex App Server process 必須在 managed execution 的生命週期中持續存在。

---

# 30. Restart / reconnect

如果 `codex-village` restart：

1. 重新找到 selected root thread。
2. 從 persisted metadata / session history 重建 parent-child graph。
3. 還原 deterministic avatar。
4. 還原 stable house / unit positions。
5. 從安全 offset 繼續 tail。
6. 不要重播完整歷史動畫。

既有 agents 應該立即以：

```text
best-known current state
```

重新出現在地圖上。

---

# 31. Tailscale

視覺化頁面必須能從使用者其他 tailnet 裝置存取。

先：

```bash
tailscale status
```

如果本機已經有 node：

**直接重用。**

不要建立第二台 hostname。

取得 IPv4：

```bash
tailscale ip -4
```

Go server 已經 listen：

```text
0.0.0.0:8040
```

除非使用者特別要求：

```text
只使用 HTTP
不要自行增加 HTTPS
```

測試：

```text
http://<tailscale-ip>:8040/
```

預期：

```text
HTTP 200
```

完成後，要給使用者兩個實際可點擊網址：

```text
http://<hostname>.<tailnet>.ts.net:8040
http://<100.x.x.x>:8040
```

不要只說：

> Tailscale configured.

真正網址本身就是 deliverable。

如果 Tailscale 尚未安裝，可以安裝與設定，但必須遵循主機正常的權限與登入流程。

不要替使用者輸入 credentials。

---

# 32. Tests

執行：

```bash
go test ./...
```

至少涵蓋以下測試。

---

## JSONL tailing

測試：

* initial EOF behavior
* offsets
* partial final line
* append
* truncation
* file rotation / replacement
* malformed JSON
* unknown event type

---

## Codex normalization

測試：

* root session detection
* subagent spawn
* nested subagent spawn
* parent-child preservation
* running
* completed
* interrupted
* errored
* waiting state
* tool activity
* reasoning activity

並確認：

```text
reasoning content 本身不會被洩漏
```

---

## Coalescing

測試：

* burst collapse
* 不同 agent 的 burst 不會互相 merge
* state-changing events 不會被丟失

---

## App Server

測試：

* partial stdout lines
* malformed protocol lines
* request / response correlation
* turn/start
* turn/steer
* stale expectedTurnId rejection
* child prompting unsupported fallback
* process exit
* reconnect behavior

---

## Security

建立 synthetic secret data，確認 frontend event 絕對不含：

* API keys
* Authorization headers
* raw shell command
* raw hidden reasoning
* raw transcript payload

---

## UI model

測試：

* roster spawn
* completed agent remains visible
* idle → sleep
* new activity wakes sleeping agent
* stable positions
* deterministic avatars

---

# 33. README

README 必須說明：

```text
What codex-village visualizes
Architecture
Observer mode
Managed mode
Demo mode
CODEX_HOME
Selecting a root thread
Codex App Server integration
Prompting / steering
Security boundary
Tailscale
Keyboard shortcuts
Tests
```

並加入 architecture diagram：

```text
                       Codex
                         │
          ┌──────────────┴──────────────┐
          │                             │
     App Server                   rollout JSONL
   live structured                    fallback /
       events                         observer
          │                             │
          └──────────────┬──────────────┘
                         ▼
                  normalization
                         │
                         ▼
                  Go world state
                         │
                    WebSocket
                         │
                         ▼
                480×270 Canvas
                         │
          ┌──────────────┴─────────────┐
          ▼                            ▼
      subagents                     player
```

---

# 34. 最重要的產品原則

這個專案不是：

```text
transcript viewer + pixel art
```

真正的目標是：

```text
Codex execution
      ↓
spatial mental model
```

使用者應該只看地圖就能理解：

```text
誰正在工作？
誰 spawn 了誰？
誰正在使用 tools？
誰正在 thinking？
誰需要我介入？
誰完成了？
誰失敗了？
誰正在 idle？
```

在使用者閱讀文字之前：

**空間本身就必須能表達 execution structure。**

---

# 35. V1 不要過度開發

優先級：

```text
P0  detect selected Codex execution tree
P0  distinguish root vs subagents
P0  real-time state normalization
P0  spawn / work / idle / complete animation
P0  nested delegation visualization
P0  click slim card
P0  observer mode
P0  --demo

P1  managed App Server mode
P1  player prompting / turn steering
P1  Tailscale

P2  richer tool-specific animations
P2  extra environment decoration
P2  historical replay
P2  analytics
```

不要為了先把畫面做漂亮，而犧牲可靠的 Codex state tracking。

---

# 36. Definition of Done

以下流程全部正常，才算完成。

1. 啟動一個 Codex root agent。
2. Root spawn 三個 subagents。
3. 地圖不需 refresh，就出現三個 units。
4. 其中一個 subagent 使用 tool，角色走到工作區並開始工作動畫。
5. 一個 subagent 又 spawn nested child。
6. 地圖清楚顯示這個 parent-child 關係。
7. 一個 agent 等待 human input，畫面上有明顯 attention tell。
8. 一個 agent 完成後仍留在畫面，變成 idle。
9. 長時間 quiet 的 agent 會睡覺，但不消失。
10. 點擊 agent 可以打開 slim card，而且 map 不會移動。
11. Managed mode 下，選取支援 direct prompting 的 thread，發送 message 後，player 會走向該 agent，並透過 Codex App Server 送出訊息。
12. Browser 永遠看不到：

    * raw transcript
    * hidden reasoning
    * secret
    * command payload
    * sensitive tool output
13. `go test ./...` 全部通過。
14. 使用 Tailscale IPv4 存取網站時回傳 HTTP 200。
15. 使用者最後拿到兩個實際可使用的 tailnet URL。

最後 ship 到：

```text
main
```
