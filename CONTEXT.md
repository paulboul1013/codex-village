# Context

## Glossary

### Thread

A real execution thread owned and emitted by Codex. A root thread is the selected execution-tree origin; a child thread is a thread spawned by another thread.

### AgentNode

The backend's normalized visual projection of one Codex Thread. An AgentNode carries the stable identity, parent relationship, lifecycle state, and current activity needed by the world view.

### Root agent

The orchestrating Codex Thread at the root of the selected execution tree. It is represented as an AgentNode with distinct visual treatment.

### Subagent

A child Codex Thread represented by an AgentNode below another AgentNode in the execution tree.

### Player

The human user interacting with codex-village. The Player is not a Thread, AgentNode, root agent, or subagent, and is not part of the agent roster.

### Execution tree

The selected root Thread together with only its true descendants. Unrelated historical or concurrent Codex sessions are not part of the tree.

### Selected root Thread

The Thread chosen as the origin of the current world view. An explicit Thread ID may select a child Thread as the view root; ancestors outside that selected boundary are not rendered.

### Pending relation

A candidate Thread whose parent relationship has been observed but not yet validated or whose parent is not yet discoverable. A pending relation is not attached to the execution tree by heuristic inference.

### Lifecycle state

The task/execution state of an AgentNode: pending initialization, running, waiting for input or approval, completed, interrupted, or errored. Idle and sleeping are presence states, not replacements for a terminal lifecycle state.

### Activity kind

The current kind of work associated with an AgentNode: reasoning, tool use, delegation, inspection, testing, research, git, review, or unknown. Activity kind is separate from lifecycle state and is allowlisted before it reaches the frontend.

### Presence

The world-visible availability of an AgentNode: active, idle, or sleeping. Presence is derived from recent activity and may coexist with any lifecycle state, including completed.

### Attention state

The human-attention need of an AgentNode: none, waiting for input, or waiting for approval. Attention is separate from lifecycle, activity, and presence, and waiting attention must remain visible even when the world is quiet.

### Prompt intent

A single human message addressed to a selected AgentNode through Managed mode. A prompt intent is not a transcript view and does not grant permission to expose the agent's private input, tool data, or reasoning.

### Approval request

A Codex interaction that requires an explicit human approval decision. An approval request is distinct from a Prompt intent; sending a normal message does not satisfy it.

### Normalized event

A stable backend event derived from Codex raw protocol or rollout input. The frontend consumes normalized events and never depends on raw Codex payloads.

### World snapshot

The current safe normalized state of the selected execution tree: AgentNodes, relationships, lifecycle, activity, presence, attention, positions, and avatars. A snapshot rehydrates the world without replaying historical animations.

### File generation

The identity-bound lifetime of one observed rollout file representation. A new inode/device identity, truncation, or replacement starts a new generation and requires rebuilding that file's selected-tree contribution.

### Observer mode

A read-only visualization mode that observes an existing Codex session through local rollout data. Direct prompting is disabled by default.

### Managed mode

A mode in which codex-village owns a long-lived Codex App Server child process and controls a root Thread through structured JSON-RPC over stdin/stdout.

### Delegation

The parent-to-child relationship created when one Codex Thread spawns another. The world must make this relationship spatially visible.
