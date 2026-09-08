package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverRolloutThreadsExtractsOnlyAllowlistedMetadata(t *testing.T) {
	home := t.TempDir()
	day := filepath.Join(home, "sessions", "2026", "09", "08")
	if err := os.MkdirAll(day, 0o700); err != nil {
		t.Fatal(err)
	}
	writeRolloutFixture(t, filepath.Join(day, "rollout-root.jsonl"), `
{"timestamp":"2026-09-08T10:00:00Z","type":"session_meta","payload":{"id":"root","cwd":"/workspace/app","timestamp":"2026-09-08T10:00:00Z","base_instructions":"private instructions","source":"cli"}}
{"timestamp":"2026-09-08T10:02:00Z","type":"response_item","payload":{"type":"reasoning","content":"private reasoning"}}
`)
	writeRolloutFixture(t, filepath.Join(day, "rollout-child.jsonl"), `
{"timestamp":"2026-09-08T10:01:00Z","type":"session_meta","payload":{"id":"child","cwd":"/workspace/app","timestamp":"2026-09-08T10:01:00Z","agent_nickname":"explorer","agent_role":"worker","source":{"subagent":{"thread_spawn":{"parent_thread_id":"root","agent_nickname":"explorer","agent_role":"worker","depth":1}}}}}
`)

	catalog, err := discoverRolloutThreads(home)
	if err != nil {
		t.Fatalf("discover rollouts: %v", err)
	}
	if got, want := threadIDs(catalog.Records), []string{"child", "root"}; !equalStrings(got, want) {
		t.Fatalf("discovered threads = %v, want %v", got, want)
	}
	child := catalog.Records[0]
	if child.ParentThreadID != "root" || child.CWD != "/workspace/app" || child.Agent.Name != "explorer" || child.Agent.Role != "subagent" {
		t.Fatalf("child metadata = %+v", child)
	}
	root := catalog.Records[1]
	if got := root.LastActivityAt.Format("2006-01-02T15:04:05Z07:00"); got != "2026-09-08T10:02:00Z" {
		t.Fatalf("root activity = %s, want latest safe envelope timestamp", got)
	}
	if catalog.Diagnostics.MalformedLines != 0 || catalog.Diagnostics.FilesScanned != 2 {
		t.Fatalf("diagnostics = %+v", catalog.Diagnostics)
	}
}

func TestDiscoverRolloutThreadsSkipsMalformedAndUnidentifiedFiles(t *testing.T) {
	home := t.TempDir()
	day := filepath.Join(home, "sessions", "2026", "09", "08")
	if err := os.MkdirAll(day, 0o700); err != nil {
		t.Fatal(err)
	}
	writeRolloutFixture(t, filepath.Join(day, "rollout-valid.jsonl"), "not-json\n"+
		`{"timestamp":"2026-09-08T10:00:00Z","type":"unknown_future_event","payload":{"secret":"hidden"}}`+"\n"+
		`{"timestamp":"2026-09-08T10:01:00Z","type":"session_meta","payload":{"id":"valid","cwd":"/workspace/app","source":"cli"}}`+"\n")
	writeRolloutFixture(t, filepath.Join(day, "rollout-no-identity.jsonl"), `{"type":"response_item","payload":{"content":"private"}}`+"\n")

	catalog, err := discoverRolloutThreads(home)
	if err != nil {
		t.Fatalf("discover rollouts: %v", err)
	}
	if got := threadIDs(catalog.Records); len(got) != 1 || got[0] != "valid" {
		t.Fatalf("discovered threads = %v, want [valid]", got)
	}
	if catalog.Diagnostics.MalformedLines != 1 || catalog.Diagnostics.UnidentifiedFiles != 1 {
		t.Fatalf("diagnostics = %+v, want malformed and unidentified counts", catalog.Diagnostics)
	}
}

func TestDiscoverRolloutThreadsRehydratesCurrentStateWithoutRetainingPayloads(t *testing.T) {
	home := t.TempDir()
	day := filepath.Join(home, "sessions", "2026", "09", "08")
	if err := os.MkdirAll(day, 0o700); err != nil {
		t.Fatal(err)
	}
	writeRolloutFixture(t, filepath.Join(day, "rollout-worker.jsonl"),
		`{"timestamp":"2026-09-08T10:00:00Z","type":"session_meta","payload":{"id":"worker","cwd":"/workspace/app","source":"cli"}}`+"\n"+
			`{"timestamp":"2026-09-08T10:01:00Z","type":"response_item","payload":{"type":"function_call","name":"shell","arguments":"private command"}}`+"\n"+
			`{"timestamp":"2026-09-08T10:02:00Z","type":"event_msg","payload":{"type":"task_complete","last_agent_message":"private answer"}}`+"\n")

	catalog, err := discoverRolloutThreads(home)
	if err != nil {
		t.Fatalf("discover rollouts: %v", err)
	}
	if len(catalog.Records) != 1 {
		t.Fatalf("record count = %d, want 1", len(catalog.Records))
	}
	node := catalog.Records[0].Agent
	if node.LifecycleState != "completed" || node.ActivityKind != "unknown" || node.Presence != "idle" {
		t.Fatalf("rehydrated node = %+v, want completed current state", node)
	}
}

func writeRolloutFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
