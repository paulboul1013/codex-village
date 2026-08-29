package main

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestSelectExecutionRootByExplicitThreadAllowsChildBoundary(t *testing.T) {
	records := []ThreadRecord{
		fixtureThread("root", "", "session-1", "/workspace/app", "2026-08-29T10:00:00Z", "root"),
		fixtureThread("child", "root", "session-1", "/workspace/app", "2026-08-29T10:01:00Z", "subagent"),
	}

	root, err := selectExecutionRoot(records, ThreadSelector{ThreadID: "child"})
	if err != nil {
		t.Fatalf("select explicit child: %v", err)
	}
	if root.ID != "child" {
		t.Fatalf("selected root = %q, want child", root.ID)
	}

	tree, err := reconstructExecutionTree(records, root.ID)
	if err != nil {
		t.Fatalf("reconstruct selected child: %v", err)
	}
	if got := threadIDs(tree.Threads); len(got) != 1 || got[0] != "child" {
		t.Fatalf("selected child tree = %v, want [child]", got)
	}
	if tree.Threads[0].ParentThreadID != "" {
		t.Fatalf("view root parent = %q, want empty boundary parent", tree.Threads[0].ParentThreadID)
	}
}

func TestSelectExecutionRootLatestUsesDeterministicOrdering(t *testing.T) {
	records := []ThreadRecord{
		fixtureThread("older", "", "session-1", "/workspace/app", "2026-08-29T10:00:00Z", "root"),
		fixtureThread("zeta", "", "session-2", "/workspace/app", "2026-08-29T11:00:00Z", "root"),
		fixtureThread("alpha", "", "session-3", "/workspace/app", "2026-08-29T11:00:00Z", "root"),
		fixtureThread("child", "zeta", "session-2", "/workspace/app", "2026-08-29T12:00:00Z", "subagent"),
	}

	root, err := selectExecutionRoot(records, ThreadSelector{Latest: true})
	if err != nil {
		t.Fatalf("select latest root: %v", err)
	}
	if root.ID != "alpha" {
		t.Fatalf("latest root = %q, want alpha tie-breaker", root.ID)
	}
}

func TestSelectExecutionRootFiltersNormalizedCWD(t *testing.T) {
	records := []ThreadRecord{
		fixtureThread("project", "", "session-1", "/workspace/project", "2026-08-29T10:00:00Z", "root"),
		fixtureThread("other", "", "session-2", "/workspace/other", "2026-08-29T11:00:00Z", "root"),
	}

	root, err := selectExecutionRoot(records, ThreadSelector{
		Latest: true,
		CWD:    "/workspace/project/.",
	})
	if err != nil {
		t.Fatalf("select cwd root: %v", err)
	}
	if root.ID != "project" {
		t.Fatalf("cwd root = %q, want project", root.ID)
	}
}

func TestEligibleRootsForPickerExcludesChildrenAndDoesNotInferForkProvenance(t *testing.T) {
	records := []ThreadRecord{
		fixtureThread("root", "", "session-1", "/workspace/app", "2026-08-29T10:00:00Z", "root"),
		fixtureThread("child", "root", "session-1", "/workspace/app", "2026-08-29T11:00:00Z", "subagent"),
		{
			ID:             "fork",
			ForkedFromID:   "root",
			SessionID:      "session-2",
			CWD:            "/workspace/app",
			LastActivityAt: mustTime("2026-08-29T12:00:00Z"),
			Agent:          AgentNode{ID: "fork", Name: "fork", Role: "subagent"},
		},
		{CWD: "/workspace/app"},
	}

	roots := eligibleRoots(records, "/workspace/app")
	if got, want := threadIDs(roots), []string{"fork", "root"}; !equalStrings(got, want) {
		t.Fatalf("picker roots = %v, want %v", got, want)
	}
}

func TestSelectExecutionRootReturnsClearErrors(t *testing.T) {
	records := []ThreadRecord{
		fixtureThread("root", "", "session-1", "/workspace/app", "2026-08-29T10:00:00Z", "root"),
	}

	if _, err := selectExecutionRoot(records, ThreadSelector{ThreadID: "missing"}); !errors.Is(err, ErrThreadNotFound) {
		t.Fatalf("missing explicit thread error = %v, want ErrThreadNotFound", err)
	}
	if _, err := selectExecutionRoot(records, ThreadSelector{ThreadID: "root", CWD: "/workspace/other"}); !errors.Is(err, ErrCWDMismatch) {
		t.Fatalf("cwd mismatch error = %v, want ErrCWDMismatch", err)
	}
	if _, err := selectExecutionRoot(records, ThreadSelector{}); !errors.Is(err, ErrSelectorRequired) {
		t.Fatalf("missing selector error = %v, want ErrSelectorRequired", err)
	}
}

func TestReconstructExecutionTreeKeepsNestedTerminalDescendantsAndRejectsUnrelatedSessions(t *testing.T) {
	records := []ThreadRecord{
		fixtureThread("root", "", "session-1", "/workspace/app", "2026-08-29T10:00:00Z", "root"),
		fixtureThread("child", "root", "session-1", "/workspace/app", "2026-08-29T10:01:00Z", "subagent"),
		{
			ID:             "grandchild",
			ParentThreadID: "child",
			SessionID:      "session-1",
			CWD:            "/workspace/app",
			LastActivityAt: mustTime("2026-08-29T10:02:00Z"),
			Agent: AgentNode{
				ID:             "grandchild",
				Name:           "grandchild",
				Role:           "subagent",
				LifecycleState: "completed",
			},
		},
		fixtureThread("unrelated", "", "session-2", "/workspace/app", "2026-08-29T13:00:00Z", "root"),
		fixtureThread("unrelated-child", "unrelated", "session-2", "/workspace/app", "2026-08-29T13:01:00Z", "subagent"),
		{
			ID:           "fork-only",
			ForkedFromID: "root",
			SessionID:    "session-1",
			CWD:          "/workspace/app",
			Agent:        AgentNode{ID: "fork-only", Name: "fork-only", Role: "subagent"},
		},
	}

	tree, err := reconstructExecutionTree(records, "root")
	if err != nil {
		t.Fatalf("reconstruct tree: %v", err)
	}
	if got, want := threadIDs(tree.Threads), []string{"root", "child", "grandchild"}; !equalStrings(got, want) {
		t.Fatalf("tree threads = %v, want %v", got, want)
	}
	if got, want := edgePairs(tree.Edges), []string{"root->child", "child->grandchild"}; !equalStrings(got, want) {
		t.Fatalf("tree edges = %v, want %v", got, want)
	}
	if tree.Threads[2].Agent.LifecycleState != "completed" {
		t.Fatalf("terminal descendant lifecycle = %q, want completed", tree.Threads[2].Agent.LifecycleState)
	}
}

func TestReconstructExecutionTreeKeepsInvalidRelationsPending(t *testing.T) {
	records := []ThreadRecord{
		fixtureThread("root", "", "session-1", "/workspace/app", "2026-08-29T10:00:00Z", "root"),
		fixtureThread("missing-parent-child", "missing-parent", "session-1", "/workspace/app", "2026-08-29T10:01:00Z", "subagent"),
		fixtureThread("session-mismatch", "root", "session-2", "/workspace/app", "2026-08-29T10:02:00Z", "subagent"),
		fixtureThread("conflict", "root", "session-1", "/workspace/app", "2026-08-29T10:03:00Z", "subagent"),
		fixtureThread("conflict", "other-parent", "session-1", "/workspace/app", "2026-08-29T10:03:00Z", "subagent"),
	}

	tree, err := reconstructExecutionTree(records, "root")
	if err != nil {
		t.Fatalf("reconstruct pending tree: %v", err)
	}
	if len(tree.Threads) != 1 {
		t.Fatalf("pending tree thread count = %d, want root only", len(tree.Threads))
	}
	if got, want := pendingPairs(tree.PendingRelations), []string{
		"conflict->other-parent",
		"conflict->root",
		"missing-parent-child->missing-parent",
		"session-mismatch->root",
	}; !equalStrings(got, want) {
		t.Fatalf("pending relations = %v, want %v", got, want)
	}
}

func TestExecutionTreeSnapshotContainsOnlySafeWorldFields(t *testing.T) {
	records := []ThreadRecord{
		fixtureThread("root", "", "private-session", "/private/workspace", "2026-08-29T10:00:00Z", "root"),
		fixtureThread("child", "root", "private-session", "/private/workspace", "2026-08-29T10:01:00Z", "subagent"),
	}
	tree, err := reconstructExecutionTree(records, "root")
	if err != nil {
		t.Fatalf("reconstruct safe tree: %v", err)
	}

	encoded, err := json.Marshal(worldSnapshot(tree))
	if err != nil {
		t.Fatalf("marshal tree snapshot: %v", err)
	}
	for _, unsafeField := range []string{"sessionID", "parentThreadId", "forkedFromId", "cwd", "private"} {
		if strings.Contains(string(encoded), unsafeField) {
			t.Fatalf("tree snapshot contains unsafe field %q: %s", unsafeField, encoded)
		}
	}
	for _, safeField := range []string{"\"type\":\"snapshot\"", "\"agents\"", "\"parentId\":\"root\""} {
		if !strings.Contains(string(encoded), safeField) {
			t.Fatalf("tree snapshot missing safe field %q: %s", safeField, encoded)
		}
	}
}

func fixtureThread(id, parentID, sessionID, cwd, lastActivity, role string) ThreadRecord {
	return ThreadRecord{
		ID:             id,
		ParentThreadID: parentID,
		SessionID:      sessionID,
		CWD:            cwd,
		LastActivityAt: mustTime(lastActivity),
		Agent: AgentNode{
			ID:             id,
			Name:           id,
			Role:           role,
			LifecycleState: "running",
			ActivityKind:   "reasoning",
			Presence:       "active",
		},
	}
}

func mustTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(err)
	}
	return parsed
}

func threadIDs(records []ThreadRecord) []string {
	ids := make([]string, 0, len(records))
	for _, record := range records {
		ids = append(ids, record.ID)
	}
	return ids
}

func edgePairs(edges []ThreadEdge) []string {
	pairs := make([]string, 0, len(edges))
	for _, edge := range edges {
		pairs = append(pairs, edge.ParentID+"->"+edge.ChildID)
	}
	return pairs
}

func pendingPairs(relations []PendingRelation) []string {
	pairs := make([]string, 0, len(relations))
	for _, relation := range relations {
		pairs = append(pairs, relation.ChildID+"->"+relation.ParentID)
	}
	return pairs
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
