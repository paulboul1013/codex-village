Original prompt: 建立 codex-village，以空間化世界即時視覺化 Codex root agent 與 subagents。

## Product direction

- Selected visual style: Warcraft III-like RTS base, without copying proprietary assets or trademarks.
- Root agents read as command structures; subagents read as worker units.
- Rollout data must remain read-only and raw sensitive payloads must never reach the browser.

## Completed

- Demo server, normalized world snapshot, and execution-tree selection.
- Observer rollout catalog and safe JSONL tailing.
- Live activity normalization and WebSocket snapshot updates.

## Current slice

- Rendered live agent lifecycle/activity as an RTS base with deterministic animation hooks.
- Added animated headquarters, worker units, hierarchy paths, reasoning pulse, tool hammer/sparks, research orb, terminal state markers, fullscreen toggle, and connection status.
- Frontend contract test passes; Playwright deterministic state, normal/fullscreen screenshots, and a clean browser console were verified.
- The skill client needed an isolated `.mjs` copy and the installed system Chrome because its bundled ESM/browser assumptions did not match this environment; no repository dependency was added.

## TODO

- Discover newly spawned rollout files while Observer mode is already running.
- Normalize waiting, approval, explicit failure, delegation, and nested spawn events.
- Add reconnect and quiet/idle timing behavior.
- Build Managed mode through Codex App Server.
