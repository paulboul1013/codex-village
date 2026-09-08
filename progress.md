Original prompt: 建立 codex-village，以空間化世界即時視覺化 Codex root agent 與 subagents。

## Product direction

- Selected visual style: Warcraft III-like RTS base, without copying proprietary assets or trademarks.
- Root agents read as command structures; subagents read as worker units.
- Rollout data must remain read-only and raw sensitive payloads must never reach the browser.

## Completed

- Demo server, normalized world snapshot, and execution-tree selection.
- Observer rollout catalog and safe JSONL tailing.
- Live activity normalization and WebSocket snapshot updates.
- Live rollout discovery attaches newly spawned and nested descendants to the selected tree without admitting unrelated sessions.
- Newly discovered files rehydrate one current safe state without replaying historical events as animations.

## Current slice

- Rendered live agent lifecycle/activity as an RTS base with deterministic animation hooks.
- Added animated headquarters, worker units, hierarchy paths, reasoning pulse, tool hammer/sparks, research orb, terminal state markers, fullscreen toggle, and connection status.
- Frontend contract test passes; Playwright deterministic state, normal/fullscreen screenshots, and a clean browser console were verified.
- The skill client needed an isolated `.mjs` copy and the installed system Chrome because its bundled ESM/browser assumptions did not match this environment; no repository dependency was added.

## TODO

- Normalize waiting, approval, explicit failure, delegation, and nested spawn events.
- Add reconnect and quiet/idle timing behavior.
- Build Managed mode through Codex App Server.

## Three-subagent live test (2026-09-08)

- Spawned three real read-only subagents while `go run . --latest` was already running.
- WSL health endpoint stayed healthy at the dynamically resolved `eth0` URL.
- `/api/tree?latest=true` and two Playwright frames both contained only the root agent (`agentCount: 1`).
- Human inspection of both screenshots confirmed that no worker units appeared or moved.
- Root cause: the current Observer source catalogs rollout files only at startup; its polling loop tails only files already selected then. Implement live rollout discovery before treating real subagent spawn animation as working.
- Resolution implemented after this test: poll for new rollout paths once per second, validate parent relationships, attach true descendants, and broadcast the revised snapshot. A fresh three-subagent visual test still requires restarting the older running server binary.
