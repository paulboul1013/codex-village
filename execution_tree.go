package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ThreadRecord is the normalized metadata needed to select and reconstruct a
// single execution tree. ForkedFromID is provenance only; it never creates an
// execution-tree edge.
type ThreadRecord struct {
	ID             string
	ParentThreadID string
	SessionID      string
	ForkedFromID   string
	CWD            string
	LastActivityAt time.Time
	Agent          AgentNode
}

type ThreadSelector struct {
	ThreadID string
	Latest   bool
	CWD      string
}

var (
	ErrThreadNotFound   = errors.New("thread not found")
	ErrNoEligibleRoot   = errors.New("no eligible root thread")
	ErrSelectorRequired = errors.New("thread selector required")
	ErrCWDMismatch      = errors.New("thread working directory mismatch")
)

type ThreadEdge struct {
	ParentID string
	ChildID  string
}

type PendingRelationReason string

const (
	PendingMissingParent     PendingRelationReason = "missing_parent"
	PendingConflictingParent PendingRelationReason = "conflicting_parent"
	PendingSessionMismatch   PendingRelationReason = "session_mismatch"
)

type PendingRelation struct {
	ChildID  string
	ParentID string
	Reason   PendingRelationReason
}

type ExecutionTree struct {
	RootID           string
	Threads          []ThreadRecord
	Edges            []ThreadEdge
	PendingRelations []PendingRelation
}

type threadIndex struct {
	byID      map[string]ThreadRecord
	conflicts map[string]bool
	parentIDs map[string][]string
}

func selectExecutionRoot(records []ThreadRecord, selector ThreadSelector) (ThreadRecord, error) {
	index := indexThreads(records)
	threadID := strings.TrimSpace(selector.ThreadID)
	if threadID != "" {
		record, ok := index.byID[threadID]
		if !ok || index.conflicts[threadID] {
			return ThreadRecord{}, fmt.Errorf("%w: %s", ErrThreadNotFound, threadID)
		}
		if selector.CWD != "" && normalizeCWD(record.CWD) != normalizeCWD(selector.CWD) {
			return ThreadRecord{}, fmt.Errorf("%w: %s", ErrCWDMismatch, threadID)
		}
		return record, nil
	}

	if selector.Latest {
		roots := eligibleRootsFromIndex(index, selector.CWD)
		if len(roots) == 0 {
			return ThreadRecord{}, fmt.Errorf("%w for cwd %q", ErrNoEligibleRoot, selector.CWD)
		}
		return roots[0], nil
	}

	return ThreadRecord{}, ErrSelectorRequired
}

// eligibleRoots returns the candidates that a no-selector Observer picker may
// display. It excludes child threads and never treats fork provenance as an
// implicit child edge.
func eligibleRoots(records []ThreadRecord, cwd string) []ThreadRecord {
	return eligibleRootsFromIndex(indexThreads(records), cwd)
}

func eligibleRootsFromIndex(index threadIndex, cwd string) []ThreadRecord {
	wantedCWD := normalizeCWD(cwd)
	roots := make([]ThreadRecord, 0)
	for id, record := range index.byID {
		if index.conflicts[id] || record.ParentThreadID != "" {
			continue
		}
		if wantedCWD != "" && normalizeCWD(record.CWD) != wantedCWD {
			continue
		}
		roots = append(roots, record)
	}

	// Latest means newest LastActivityAt, with a stable ID tie-breaker. The
	// timestamp orders eligible roots only and is never used to infer ancestry.
	sort.Slice(roots, func(left, right int) bool {
		if !roots[left].LastActivityAt.Equal(roots[right].LastActivityAt) {
			return roots[left].LastActivityAt.After(roots[right].LastActivityAt)
		}
		return roots[left].ID < roots[right].ID
	})
	return roots
}

func reconstructExecutionTree(records []ThreadRecord, rootID string) (ExecutionTree, error) {
	index := indexThreads(records)
	root, ok := index.byID[rootID]
	if !ok {
		return ExecutionTree{}, fmt.Errorf("%w: %s", ErrThreadNotFound, rootID)
	}

	children := make(map[string][]string)
	pending := make([]PendingRelation, 0)
	for childID, child := range index.byID {
		if index.conflicts[childID] {
			for _, parentID := range index.parentIDs[childID] {
				if parentID == "" {
					continue
				}
				pending = append(pending, PendingRelation{
					ChildID: childID, ParentID: parentID, Reason: PendingConflictingParent,
				})
			}
			continue
		}
		parentID := child.ParentThreadID
		if childID == rootID || parentID == "" {
			continue
		}

		parent, parentExists := index.byID[parentID]
		if !parentExists {
			pending = append(pending, PendingRelation{
				ChildID: childID, ParentID: parentID, Reason: PendingMissingParent,
			})
			continue
		}
		if index.conflicts[parentID] {
			pending = append(pending, PendingRelation{
				ChildID: childID, ParentID: parentID, Reason: PendingConflictingParent,
			})
			continue
		}
		if !sessionsCompatible(parent, child) {
			pending = append(pending, PendingRelation{
				ChildID: childID, ParentID: parentID, Reason: PendingSessionMismatch,
			})
			continue
		}
		children[parentID] = append(children[parentID], childID)
	}

	for parentID := range children {
		sort.Strings(children[parentID])
	}
	sort.Slice(pending, func(left, right int) bool {
		if pending[left].ChildID != pending[right].ChildID {
			return pending[left].ChildID < pending[right].ChildID
		}
		return pending[left].ParentID < pending[right].ParentID
	})

	root.ParentThreadID = ""
	tree := ExecutionTree{
		RootID:           rootID,
		Threads:          []ThreadRecord{root},
		PendingRelations: pending,
	}
	queue := []string{rootID}
	visited := map[string]bool{rootID: true}
	for len(queue) > 0 {
		parentID := queue[0]
		queue = queue[1:]
		for _, childID := range children[parentID] {
			if visited[childID] {
				continue
			}
			visited[childID] = true
			child := index.byID[childID]
			child.ParentThreadID = parentID
			tree.Threads = append(tree.Threads, child)
			tree.Edges = append(tree.Edges, ThreadEdge{ParentID: parentID, ChildID: childID})
			queue = append(queue, childID)
		}
	}

	return tree, nil
}

func (tree ExecutionTree) normalizedWorld() normalizedWorld {
	agents := make([]AgentNode, 0, len(tree.Threads))
	for _, thread := range tree.Threads {
		agents = append(agents, safeAgentNode(thread, tree.RootID))
	}
	return normalizedWorld{Agents: agents}
}

func safeAgentNode(thread ThreadRecord, rootID string) AgentNode {
	node := thread.Agent
	node.ID = thread.ID
	node.ParentID = thread.ParentThreadID
	if thread.ID == rootID {
		node.ParentID = ""
	}
	if node.Name == "" {
		node.Name = thread.ID
	}
	return node
}

func indexThreads(records []ThreadRecord) threadIndex {
	index := threadIndex{
		byID:      make(map[string]ThreadRecord),
		conflicts: make(map[string]bool),
		parentIDs: make(map[string][]string),
	}
	for _, record := range records {
		if !validThreadRecord(record) {
			continue
		}
		if !containsString(index.parentIDs[record.ID], record.ParentThreadID) {
			index.parentIDs[record.ID] = append(index.parentIDs[record.ID], record.ParentThreadID)
			sort.Strings(index.parentIDs[record.ID])
		}
		existing, exists := index.byID[record.ID]
		if !exists {
			index.byID[record.ID] = record
			continue
		}
		if existing.ParentThreadID != record.ParentThreadID || existing.SessionID != record.SessionID {
			index.conflicts[record.ID] = true
			continue
		}
		if record.LastActivityAt.After(existing.LastActivityAt) {
			index.byID[record.ID] = record
		}
	}
	return index
}

func validThreadRecord(record ThreadRecord) bool {
	return record.ID != "" && strings.TrimSpace(record.ID) == record.ID
}

func sessionsCompatible(parent, child ThreadRecord) bool {
	return parent.SessionID == "" || child.SessionID == "" || parent.SessionID == child.SessionID
}

func normalizeCWD(cwd string) string {
	trimmed := strings.TrimSpace(cwd)
	if trimmed == "" {
		return ""
	}
	return filepath.Clean(trimmed)
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
