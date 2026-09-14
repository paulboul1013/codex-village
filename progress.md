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
- A single process-owned Observer loop tails and discovers rollouts even with no browser connected; WebSocket clients only observe shared revisions.

## Current slice

- Rendered live agent lifecycle/activity as an RTS base with deterministic animation hooks.
- Added animated headquarters, worker units, hierarchy paths, reasoning pulse, tool hammer/sparks, research orb, terminal state markers, fullscreen toggle, and connection status.
- Frontend contract test passes; Playwright deterministic state, normal/fullscreen screenshots, and a clean browser console were verified.
- The skill client needed an isolated `.mjs` copy and the installed system Chrome because its bundled ESM/browser assumptions did not match this environment; no repository dependency was added.

## TODO

- Completed Observer semantic acceptance: input/approval attention, explicit failure, delegation activity, and parent-depth spatial layout are normalized and covered by tests.
- Add reconnect and quiet/idle timing behavior.
- Build Managed mode through Codex App Server.

## Three-subagent live test (2026-09-08)

- Spawned three real read-only subagents while `go run . --latest` was already running.
- WSL health endpoint stayed healthy at the dynamically resolved `eth0` URL.
- `/api/tree?latest=true` and two Playwright frames both contained only the root agent (`agentCount: 1`).
- Human inspection of both screenshots confirmed that no worker units appeared or moved.
- Root cause: the current Observer source catalogs rollout files only at startup; its polling loop tails only files already selected then. Implement live rollout discovery before treating real subagent spawn animation as working.
- Resolution implemented after this test: poll for new rollout paths once per second, validate parent relationships, attach true descendants, and broadcast the revised snapshot. A fresh three-subagent visual test still requires restarting the older running server binary.

## Three-agent review and visual retest (2026-09-08)

- Spawned three review agents after restarting the updated Observer server; the selected tree grew to seven agents (root, three earlier completed workers, and three active reviewers).
- Playwright captured three consecutive frames. Human inspection confirmed worker entry movement, reasoning rings, tool hammer animation, completion markers, parent paths, and a clean console.
- Completed review priority: Observer polling is decoupled from WebSocket clients into one process-owned loop; attention/failure/delegation normalization and nested spatial layout are complete.
- Observer P0 file lifecycle acceptance is complete: archived moves are rediscovered, completed missing tails retire cleanly, and replacement generations rebuild their contribution. Inherited-history boundaries and bounded metadata discovery remain later hardening work.
- Product spec needs separate Observer V1 versus full-product Definition of Done, plus one canonical HTTP API naming scheme.
