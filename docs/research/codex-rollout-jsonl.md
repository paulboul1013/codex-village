# Codex rollout JSONL: format and tailing invariants

Status: research report for Wayfinder issue #3

Branch: research/codex-rollout-jsonl

Evidence date: 2026-08-29

## Executive conclusion

The safe observer model is a read-only, newline-delimited append reader with a
byte offset and a file-identity guard. It should reconstruct one explicitly
selected thread tree from persisted metadata, then tail only those rollout
files. It must not use filename proximity, timestamps, cwd, title, or agent
nickname as a parent/child inference.

The important rules are:

1. Treat sessions/YYYY/MM/DD/rollout-*.jsonl as the primary on-disk layout,
   with archived_sessions and .jsonl.zst as supported adjacent states.
2. Treat each LF-terminated physical line as one candidate JSON record. Hold a
   final unterminated byte sequence; do not parse EOF as an implicit delimiter.
3. Persist a byte offset only after the complete line, including its LF, has
   been read. The optional semantic ordinal is not a byte offset and is not
   guaranteed to be contiguous or present.
4. On truncation, replacement, or a different device/inode, start a new file
   generation and rebuild that file's selected-tree contribution.
5. A malformed complete line or unknown event is a non-fatal rejected record:
   record diagnostics, advance past its LF, and continue. A malformed
   unterminated tail remains pending until it is completed or the file changes.
6. Use parent_thread_id for the tree edge, session_id as a consistency
   check/group key, and forked_from_id as separate fork/history provenance.
7. Keep inherited fork history and child-owned live activity separate. Use
   subagent_history_start_ordinal or the corresponding history-base metadata
   to avoid double-counting copied prefixes.
8. Normalize to an allowlisted, privacy-safe activity model. Never forward raw
   rollout lines, prompts, reasoning content, tool inputs/outputs, or command
   output to a browser/UI.

These are observer invariants. They are intentionally more defensive than an
append-only happy path because the upstream implementation also supports
archives, compression transitions, forked history, malformed-line recovery,
and versioned event unions.

## Evidence and scope

### Local read-only inspection

The local Codex session directory was inspected without opening any rollout
file for writing. At the inspection point it contained 230 plain
rollout-*.jsonl files under nested date directories, totaling approximately
233 MB; no .jsonl.zst file was present at that moment. The installed CLI was
version 0.149.0, while the sample included older metadata versions as well.
The absence of compressed files is only a local observation, not a support
claim.

Observed local record families included:

- outer session_meta, event_msg, response_item, world_state, turn_context,
  inter_agent_communication, and inter_agent_communication_metadata records;
- optional outer ordinal values in newer records, with older files lacking
  that field;
- event payloads such as task_started, task_complete, turn_aborted,
  item_completed, token_count, thread_settings_applied, and
  sub_agent_activity;
- response items such as message, reasoning, function_call,
  function_call_output, custom_tool_call, custom_tool_call_output, and
  web_search_call;
- child metadata containing a parent thread, fork source, agent metadata, and
  subagent_history_start_ordinal.

Two sibling child samples pointed at the same root parent and shared the same
history-start boundary. This is direct evidence that parentage is represented
as metadata, not something that should be guessed from the filename timestamp.

One local rollout had a final byte of 0x2c (comma), no terminal LF, and a
malformed parse result. No rollout bytes were changed. This is a useful
fixture for the pending-partial-line rule, but it does not establish how a
future writer will recover every possible damaged tail.

### Upstream source versus local observation

The persisted rollout format is an implementation format, not a timeless
public contract. The conclusions below distinguish current upstream source
behavior from observations of the locally installed CLI. Consumers should
pin fixtures and test against the Codex version they support.

## Filesystem layout and naming

The rollout source declares sessions as the active session directory and
archived_sessions as the archive directory. Thread listing documents the
date-partitioned layout:

  $CODEX_HOME/sessions/YYYY/MM/DD/rollout-YYYY-MM-DDThh-mm-ss-<thread-id>.jsonl

$CODEX_HOME defaults to ~/.codex for the normal CLI configuration. See the
upstream [rollout listing implementation](https://github.com/openai/codex/blob/main/codex-rs/rollout/src/list.rs)
and [rollout library constants](https://github.com/openai/codex/blob/main/codex-rs/rollout/src/lib.rs).

The ordinary basename is rollout-<timestamp>-<thread_id>.jsonl. A reverted
rollout can use an additional suffix:

  rollout-<timestamp>-<thread_id>_<rollout_id>.jsonl

The upstream [filename parser](https://github.com/openai/codex/blob/main/codex-rs/rollout/src/rollout_file_name.rs)
parses the timestamp as YYYY-MM-DDThh-mm-ss, then separates the thread ID
from an optional underscore-delimited rollout ID. Do not treat the timestamp
as event time or as a parent relationship.

Plain JSONL and compressed .jsonl.zst representations are supported by the
upstream [compression layer](https://github.com/openai/codex/blob/main/codex-rs/rollout/src/compression.rs).
The writer can materialize a compressed rollout back to a plain file before
appending. A watcher must therefore handle a temporary disappearance or
representation change and re-discover the logical thread instead of assuming
that one pathname remains permanent.

Archiving is also a pathname operation: the local archive implementation
renames a session into archived_sessions, and can move more than one owned
descendant. See
[archive_thread.rs](https://github.com/openai/codex/blob/main/codex-rs/thread-store/src/local/archive_thread.rs).
A path watcher must treat disappearance, rename, and same-path replacement as
normal state transitions.

## Physical record and event shapes

### Physical envelope

At local CLI 0.149.0, a physical line is a JSON object with a timestamp,
an optional ordinal, an outer type discriminator, and a type-specific payload.
For event and response records the payload has its own inner type. Metadata
records expose their metadata fields under the session-meta payload. The
exact set of fields varies by CLI version and record family.

The upstream decoder removes the outer timestamp and optional ordinal before
deserializing the logical rollout item. See
[decode_rollout_line](https://github.com/openai/codex/blob/main/codex-rs/rollout/src/lib.rs)
and the [recorder writer](https://github.com/openai/codex/blob/main/codex-rs/rollout/src/recorder.rs).
An observer should preserve the physical envelope only as internal parsing
metadata and should expose a stable normalized model of its own.

### Session metadata

session_meta is the identity and compatibility anchor. Relevant fields seen
locally and supported by upstream creation parameters include:

- thread/session identity;
- parent_thread_id for a subagent's immediate parent;
- forked_from_id for the source thread of a fork;
- history_mode and history-base information;
- subagent_history_start_ordinal;
- CLI version, cwd, source/thread-source, model provider, and other session
  context.

The recorder loader treats the first applicable session metadata as the
canonical identity for a rollout and retains later metadata as history. This
matters because a fork can contain copied parent history followed by metadata
belonging to the fork. See the upstream
[rollout loading path](https://github.com/openai/codex/blob/main/codex-rs/rollout/src/recorder.rs#L982-L1041)
and [model-context metadata handling](https://github.com/openai/codex/blob/main/codex-rs/rollout/src/model_context.rs).
Therefore, “the first physical line” and “the last metadata record” are not
interchangeable identity rules.

### Event and response families

Local files showed these useful categories:

| Family | Examples | Safe observer use |
| --- | --- | --- |
| event_msg | task_started, task_complete, turn_aborted, item_completed, token_count | Turn/task lifecycle, completion, interruption, and counters |
| event_msg collaboration | sub_agent_activity, spawn/close/waiting events | Child activity and collaboration state, after validating IDs |
| response_item | message, reasoning, function/custom tool call and output, web search call | Item existence/type/status only unless a separately approved safe field is selected |
| world_state, turn_context | persisted UI/context snapshots | Treat as internal context; do not use as the canonical tree protocol |
| metadata/communication families | session and agent communication records | Use identity fields only after allowlisting; never forward free-form content |

item_completed can contain an item object, so “one event line equals one
user-visible item” is not a safe assumption. Likewise, repeated
item_completed, task, token, and response lines can coexist for one turn.
The recorder's [load path](https://github.com/openai/codex/blob/main/codex-rs/rollout/src/recorder.rs#L982-L1097)
is the authoritative source for what the current decoder accepts.

Managed App Server JSONL is a related but distinct interface: it is
newline-delimited JSON-RPC 2.0 on stdio, with Thread -> Turn -> Item as its
core model. Its official [app-server README](https://github.com/openai/codex/blob/main/codex-rs/app-server/README.md)
documents thread/started, turn/started, item/started, item deltas,
item/completed, and turn/completed. Use that protocol when connected to
App Server; do not assume its notification envelope is identical to a
persisted rollout line.

## Root, child, fork, and activity relationships

The upstream [thread-store types](https://github.com/openai/codex/blob/main/codex-rs/thread-store/src/types.rs)
define the relationship fields used by creation and listing:

- session_id groups a root thread and its subagents in one live session;
- parent_thread_id is set for a subagent and identifies its immediate parent;
- forked_from_id identifies the source thread for a fork/history operation;
- subagent_history_start_ordinal marks where child-owned history begins after
  an inherited history prefix.

The same distinction appears in the App Server Thread model: sessionId
groups the session, parentThreadId expresses a parent, and forkedFromId
expresses fork provenance. The App Server documentation also exposes
subAgentActivity with an agentThreadId and activity kind; current source lists
kinds including started, interacted, interrupted, and completed. See
[thread protocol data](https://github.com/openai/codex/blob/main/codex-rs/app-server-protocol/src/protocol/v2/thread_data.rs)
and the [App Server protocol README](https://github.com/openai/codex/blob/main/codex-rs/app-server/README.md).

For a selected tree:

- use parent_thread_id as the only child edge;
- use session_id as a consistency/group check when present;
- annotate forked_from_id as provenance, but never turn it into a descendant
  edge;
- use explicit thread IDs and the selected tree's validated metadata as
  identity;
- leave a child pending when its parent is not yet discoverable;
- never promote a candidate because its timestamp, cwd, title, or nickname is
  close to the parent.

The fork distinction is essential: a fork can have copied parent records, but
copied history is not new child activity. A selected-tree reader should use
the history-base and subagent_history_start_ordinal boundary, and should
deduplicate by logical thread plus ordinal where ordinals exist. If a legacy
file has no usable boundary, keep inherited records marked as historical or
unknown ownership rather than attributing them to the child.

## Lifecycle and activity normalization

The following normalized states are sufficient for a safe observer:

| Normalized signal | Rollout evidence | App Server equivalent |
| --- | --- | --- |
| started | task_started and/or accepted start/item records | thread/started, turn/started, item/started |
| active | accepted activity/item delta or sub_agent_activity with interacted | item deltas and subagent activity |
| completed | task_complete, accepted completion item | item/completed, turn/completed with completed status |
| interrupted | turn_aborted, subagent interrupted activity | turn/completed with interrupted status |
| failed | explicit error/failure payload when allowlisted | turn/completed with failed status |
| unknown | missing, rejected, or unsupported lifecycle evidence | no inference from file mtime alone |

Current local samples included sub_agent_activity kinds started, interacted,
and interrupted. Upstream protocol source also documents completed, so the
consumer must accept unknown future kinds without crashing. Activity records
should carry only validated IDs, kind, timestamp/ordinal, and a safe display
label. Do not copy embedded prompt, message, command, reasoning, or
tool-result fields.

Raw reasoning is especially sensitive. App Server item data can contain
reasoning summaries/content, while persisted response items can contain
encrypted reasoning content. A rollout observer must not expose either raw
reasoning content or encrypted blobs; only an allowlisted state such as
“reasoning item observed” may cross the product boundary.

## Tailing invariants

### Reader state

Maintain per logical file:

- logical thread ID and current pathname;
- device/inode (or the platform's equivalent file identity);
- committed byte offset;
- pending bytes after the last LF;
- a generation number and parse diagnostics.

The offset is a byte offset in the physical file. It is not a line count,
ordinal, timestamp, or file modification time. Keep it separately for every
file in the selected tree.

### Append and partial lines

The upstream writer serializes one JSON value and appends an LF, flushing the
writer after the record. The [JSONL writer](https://github.com/openai/codex/blob/main/codex-rs/rollout/src/recorder.rs#L1803-L1890)
also repairs a nonempty existing file that is not newline-terminated before
appending. This supports a simple observer rule:

1. Read bytes from the committed offset.
2. Split only at LF bytes.
3. Parse each complete line, including an optional CR trim for CRLF input.
4. Advance the committed offset through the LF whether the record is accepted
   or rejected.
5. Retain bytes after the last LF and retry them on the next poll.

A valid JSON object at EOF without an LF is still pending. This avoids
publishing a record that may be incomplete and avoids losing the suffix when
the writer later appends the rest. The reader itself must never repair or
append to the rollout.

On initial startup, read enough metadata to identify the selected tree and
then begin the live tail at the end of each already-known selected file.
Rebuild only the metadata and bounded recent lifecycle state required for the
selected snapshot. A full historical replay should be an explicit separate
operation, not an accidental consequence of starting the live observer.

### Malformed JSON and unknown events

The upstream loader reads line by line, skips blank lines, records JSON parse
errors, and continues loading valid records. Its recorder tests also cover
unknown metadata compatibility cases and valid/unsafe unterminated tails. See
[recorder tests](https://github.com/openai/codex/blob/main/codex-rs/rollout/src/recorder_tests.rs)
and [load_rollout_items](https://github.com/openai/codex/blob/main/codex-rs/rollout/src/recorder.rs#L982-L1097).

Apply the following policy:

- blank complete line: ignore and advance;
- malformed complete line: quarantine/count it, advance through LF, continue;
- malformed unterminated tail: hold it; retry after append or reset after a
  file identity/size transition;
- unknown outer or inner event: preserve the byte offset, record a diagnostic,
  and ignore its semantics;
- unknown metadata required for safe identity: keep the selected explicit
  thread but degrade its relationship/lifecycle state and surface a diagnostic;
- unknown history_mode: do not invent history semantics. The current upstream
  compatibility test treats unknown fork-source history metadata as a
  rejected/diagnostic case while preserving valid records, so a consumer
  should isolate that metadata failure rather than discard the whole tree.

No parse or schema error should terminate all selected-tree watchers.

### Truncation, rotation, replacement, and archive moves

Before reading, stat the path and compare both file identity and size:

- same identity and size greater than or equal to the offset: read appended
  bytes;
- same identity and size less than the offset: treat as truncation, discard
  pending bytes, start a new generation at offset zero, and rebuild;
- different identity at the same pathname: treat as replacement/rotation,
  start a new generation, and rebuild;
- pathname missing: re-scan active and archived roots, compression siblings,
  and known thread metadata before deciding that the thread disappeared.

If an old inode remains open after a rename, finish only the bytes already
owned by that identity if the product explicitly tracks it; do not silently
join an arbitrary new file that reused the old pathname. A renamed archive
file should be matched by validated thread/session metadata, not by timestamp
proximity.

The upstream reverse scanner freezes an end byte offset and remains usable
after a rejected JSON record; this is a useful model for bounded startup
inspection and tail recovery. See
[reverse_jsonl_scanner.rs](https://github.com/openai/codex/blob/main/codex-rs/rollout/src/reverse_jsonl_scanner.rs).

### Ordinals and restart behavior

Ordinals are semantic rollout positions. They can be absent in legacy history,
can have gaps, and can be copied across fork history. Upstream resume logic
scans from the end, ignores rejected records, and continues from the final
valid ordinal plus one; it does not require a contiguous sequence. See
[ordinal.rs](https://github.com/openai/codex/blob/main/codex-rs/rollout/src/ordinal.rs).

For the observer:

- use byte offsets for exactly-once physical consumption within one file
  generation;
- optionally use (thread_id, ordinal) for semantic deduplication;
- never seek to an ordinal as if it were a byte position;
- do not reject a file merely because ordinals are missing or non-contiguous;
- after restart, restore offsets only if file identity still matches;
- if identity cannot be proven, rebuild the selected file and deduplicate
  normalized state by stable IDs/ordinals where available.

## Safe selected-tree reconstruction

The following algorithm is intentionally narrow:

1. Discover only valid rollout candidates under $CODEX_HOME/sessions, plus
   archived_sessions when historical/archive continuity is needed. Parse
   filenames with the upstream grammar; do not glob arbitrary files.
2. Read metadata-only prefixes or bounded metadata scans. Build a map keyed by
   validated thread ID, recording session ID, parent ID, fork source, history
   boundary, path, and file identity. Do not load every historical message.
3. Select the root by an explicit thread ID when supplied. For a “latest”
   selector, use the documented listing order only as the selector's defined
   ordering; do not call it “currently active” without lifecycle evidence.
4. Start the included set with that root. Traverse only records whose
   parent_thread_id is an included thread. When both sides have session_id,
   require them to agree. Keep forked_from_id as an annotated edge outside the
   child tree.
5. If a referenced child is not yet present, keep a pending relation and
   re-scan on the next notification/poll. Do not attach a same-time candidate.
6. For each included file, create an independent tail state and process only
   newline-complete records. Use the child's history boundary to avoid
   treating copied parent prefix records as fresh child activity.
7. When a validated spawn/activity record names a new child, add it only after
   its metadata validates the parent/session relation. Then create a new
   reader state; nested descendants use the same rule.
8. Reduce accepted records into a snapshot containing only safe IDs, labels,
   lifecycle states, timestamps/ordinals, relationship edges, and aggregate
   counts. Keep rejected/unknown diagnostics separate from user-visible
   content.

The result is one selected execution tree, not a global session search. A
missing or malformed child should produce an incomplete/unknown child state,
not a guessed sibling or a fabricated parent.

## Failure matrix

| Condition | Reader action | Tree/state action |
| --- | --- | --- |
| Initial file at EOF | Establish identity and EOF offset | Start live state from validated metadata |
| Appended complete line | Parse and commit through LF | Apply allowlisted event |
| Appended partial line | Retain bytes | No state change yet |
| Partial line completed | Parse once LF arrives | Apply or reject exactly once |
| Malformed complete line | Diagnostic, commit through LF | Continue selected tree |
| Unknown event type | Diagnostic, commit through LF | Ignore semantics; keep watcher alive |
| File shrinks below offset | New generation from zero | Rebuild that file's contribution |
| Same path, new identity | New generation from zero | Rebuild; do not join old/new streams |
| Path renamed to archive | Re-discover by metadata/identity | Keep only if it still belongs to selected tree |
| Plain/compressed transition | Retry/re-scan representation | Do not assume the old path is permanent |
| Missing parent metadata | Pending relation | Never infer by timestamp |
| Forked copied prefix | Apply history boundary/dedup | Do not double-count child activity |
| Unknown lifecycle state | Diagnostic | Surface unknown, not active/completed by guess |

## Validation implications

An implementation following this report should test at least:

- initial EOF and restart with a saved byte offset;
- append of one complete record;
- append in multiple partial writes;
- valid JSON without final LF;
- malformed complete JSON and malformed partial JSON;
- unknown outer and inner event types;
- truncation and same-path replacement;
- rename into archived_sessions;
- plain/compressed representation transition;
- legacy files without ordinals;
- ordinal gaps and fork history boundaries;
- root, child, nested child, and unrelated sibling selection;
- started/completed/interrupted/failed/unknown lifecycle states;
- no forwarding of reasoning, prompts, tool input/output, or command output.

For App Server integrations, generate or consume the schema matching the
running Codex version. The official README describes version-specific schema
generation; do not use a schema from a different CLI release as a persisted
rollout contract.

## Sources

All external sources below are official Codex repository source or
documentation:

- [rollout library](https://github.com/openai/codex/blob/main/codex-rs/rollout/src/lib.rs)
- [rollout recorder and writer](https://github.com/openai/codex/blob/main/codex-rs/rollout/src/recorder.rs)
- [rollout recorder tests](https://github.com/openai/codex/blob/main/codex-rs/rollout/src/recorder_tests.rs)
- [filename parser](https://github.com/openai/codex/blob/main/codex-rs/rollout/src/rollout_file_name.rs)
- [session listing](https://github.com/openai/codex/blob/main/codex-rs/rollout/src/list.rs)
- [ordinal handling](https://github.com/openai/codex/blob/main/codex-rs/rollout/src/ordinal.rs)
- [reverse JSONL scanner](https://github.com/openai/codex/blob/main/codex-rs/rollout/src/reverse_jsonl_scanner.rs)
- [plain/compressed rollout handling](https://github.com/openai/codex/blob/main/codex-rs/rollout/src/compression.rs)
- [thread-store types](https://github.com/openai/codex/blob/main/codex-rs/thread-store/src/types.rs)
- [archive implementation](https://github.com/openai/codex/blob/main/codex-rs/thread-store/src/local/archive_thread.rs)
- [App Server README](https://github.com/openai/codex/blob/main/codex-rs/app-server/README.md)
- [App Server Thread protocol data](https://github.com/openai/codex/blob/main/codex-rs/app-server-protocol/src/protocol/v2/thread_data.rs)
