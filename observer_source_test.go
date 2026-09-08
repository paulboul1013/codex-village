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

func TestObserverSourcePollStartsFromRehydratedStateWithoutHistoricalEventReplay(t *testing.T) {
	home := t.TempDir()
	day := filepath.Join(home, "sessions", "2026", "09", "08")
	if err := os.MkdirAll(day, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(day, "rollout-root.jsonl")
	writeRolloutFixture(t, path,
		`{"timestamp":"2026-09-08T10:00:00Z","type":"session_meta","payload":{"id":"root","cwd":"/workspace/app","source":"cli"}}`+"\n"+
			`{"timestamp":"2026-09-08T10:01:00Z","type":"event_msg","payload":{"type":"task_complete","last_agent_message":"historical private answer"}}`+"\n")

	source, err := openObserverSource(home, ThreadSelector{ThreadID: "root"})
	if err != nil {
		t.Fatalf("open observer source: %v", err)
	}
	if source.normalizedWorld().Agents[0].LifecycleState != "completed" {
		t.Fatalf("current state was not rehydrated: %+v", source.normalizedWorld().Agents[0])
	}
	appendFile(t, path, `{"timestamp":"2026-09-08T10:02:00Z","type":"event_msg","payload":{"type":"task_started","turn_id":"private"}}`+"\n")

	changed, err := source.poll()
	if err != nil {
		t.Fatalf("poll observer: %v", err)
	}
	if !changed {
		t.Fatal("appended activity did not change the observer world")
	}
	node := source.normalizedWorld().Agents[0]
	if node.LifecycleState != "running" || node.ActivityKind != "reasoning" || node.Presence != "active" {
		t.Fatalf("live node = %+v, want running reasoning activity", node)
	}
	if changed, err := source.poll(); err != nil || changed {
		t.Fatalf("empty poll = changed %v error %v, want no change", changed, err)
	}
}

func TestObserverSourceDiscoversNewChildRolloutAndTailsItsActivity(t *testing.T) {
	home := t.TempDir()
	day := filepath.Join(home, "sessions", "2026", "09", "08")
	if err := os.MkdirAll(day, 0o700); err != nil {
		t.Fatal(err)
	}
	writeRolloutFixture(t, filepath.Join(day, "rollout-root.jsonl"), `{"timestamp":"2026-09-08T10:00:00Z","type":"session_meta","payload":{"id":"root","cwd":"/workspace/app","source":"cli"}}`+"\n")
	source, err := openObserverSource(home, ThreadSelector{ThreadID: "root"})
	if err != nil {
		t.Fatalf("open observer source: %v", err)
	}

	childPath := filepath.Join(day, "rollout-child.jsonl")
	writeRolloutFixture(t, childPath, `{"timestamp":"2026-09-08T10:01:00Z","type":"session_meta","payload":{"id":"child","cwd":"/workspace/app","agent_nickname":"worker-1","source":{"subagent":{"thread_spawn":{"parent_thread_id":"root","agent_nickname":"worker-1"}}}}}`+"\n")
	changed, err := source.discoverNewRollouts()
	if err != nil {
		t.Fatalf("discover new child: %v", err)
	}
	if !changed {
		t.Fatal("new child did not change observer world")
	}
	agents := source.normalizedWorld().Agents
	if len(agents) != 2 || agents[1].ID != "child" || agents[1].ParentID != "root" || agents[1].Role != "subagent" {
		t.Fatalf("world after discovery = %+v, want attached child", agents)
	}

	appendFile(t, childPath, `{"timestamp":"2026-09-08T10:02:00Z","type":"response_item","payload":{"type":"function_call","name":"shell","arguments":"private"}}`+"\n")
	changed, err = source.poll()
	if err != nil {
		t.Fatalf("poll child activity: %v", err)
	}
	if !changed || source.normalizedWorld().Agents[1].ActivityKind != "tool" {
		t.Fatalf("child activity = %+v, want live tool state", source.normalizedWorld().Agents[1])
	}
}

func TestObserverSourceLeavesUnrelatedNewRolloutOutsideSelectedTree(t *testing.T) {
	home := t.TempDir()
	day := filepath.Join(home, "sessions", "2026", "09", "08")
	if err := os.MkdirAll(day, 0o700); err != nil {
		t.Fatal(err)
	}
	writeRolloutFixture(t, filepath.Join(day, "rollout-root.jsonl"), `{"timestamp":"2026-09-08T10:00:00Z","type":"session_meta","payload":{"id":"root","cwd":"/workspace/app","source":"cli"}}`+"\n")
	source, err := openObserverSource(home, ThreadSelector{ThreadID: "root"})
	if err != nil {
		t.Fatalf("open observer source: %v", err)
	}
	writeRolloutFixture(t, filepath.Join(day, "rollout-unrelated.jsonl"), `{"timestamp":"2026-09-08T10:01:00Z","type":"session_meta","payload":{"id":"unrelated","cwd":"/workspace/app","source":"cli"}}`+"\n")

	changed, err := source.discoverNewRollouts()
	if err != nil {
		t.Fatalf("discover unrelated rollout: %v", err)
	}
	if changed || len(source.normalizedWorld().Agents) != 1 {
		t.Fatalf("unrelated rollout changed selected world: %+v", source.normalizedWorld().Agents)
	}
}
