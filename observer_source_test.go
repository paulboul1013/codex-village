package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenObserverSourceBuildsSelectedWorldFromRollouts(t *testing.T) {
	home := t.TempDir()
	day := filepath.Join(home, "sessions", "2026", "09", "08")
	if err := os.MkdirAll(day, 0o700); err != nil {
		t.Fatal(err)
	}
	writeRolloutFixture(t, filepath.Join(day, "rollout-root.jsonl"), `{"timestamp":"2026-09-08T10:00:00Z","type":"session_meta","payload":{"id":"root","cwd":"/workspace/app","source":"cli"}}`+"\n")
	writeRolloutFixture(t, filepath.Join(day, "rollout-child.jsonl"), `{"timestamp":"2026-09-08T10:01:00Z","type":"session_meta","payload":{"id":"child","cwd":"/workspace/app","source":{"subagent":{"thread_spawn":{"parent_thread_id":"root","agent_nickname":"worker"}}}}}`+"\n")

	source, err := openObserverSource(home, ThreadSelector{Latest: true, CWD: "/workspace/app"})
	if err != nil {
		t.Fatalf("open observer source: %v", err)
	}
	snapshot := worldSnapshot(source)
	if len(snapshot.Agents) != 2 || snapshot.Agents[0].ID != "root" || snapshot.Agents[1].ParentID != "root" {
		t.Fatalf("observer snapshot = %+v, want selected root and child", snapshot)
	}
}

func TestOpenObserverSourceWithoutSelectorSupportsRootPicker(t *testing.T) {
	home := t.TempDir()
	day := filepath.Join(home, "sessions", "2026", "09", "08")
	if err := os.MkdirAll(day, 0o700); err != nil {
		t.Fatal(err)
	}
	writeRolloutFixture(t, filepath.Join(day, "rollout-root.jsonl"), `{"timestamp":"2026-09-08T10:00:00Z","type":"session_meta","payload":{"id":"root","cwd":"/workspace/app","source":"cli"}}`+"\n")

	source, err := openObserverSource(home, ThreadSelector{})
	if err != nil {
		t.Fatalf("open observer source: %v", err)
	}
	if len(source.threadRecords()) != 1 || len(source.normalizedWorld().Agents) != 0 {
		t.Fatalf("unselected observer source = records %d agents %d, want picker data and empty world", len(source.threadRecords()), len(source.normalizedWorld().Agents))
	}
}
