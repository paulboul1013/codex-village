package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type rolloutCatalogDiagnostics struct {
	FilesScanned      int
	MalformedLines    int
	UnidentifiedFiles int
}

type rolloutCatalog struct {
	Records     []ThreadRecord
	Diagnostics rolloutCatalogDiagnostics
}

type rolloutEnvelope struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type rolloutSessionMetadata struct {
	ID             string          `json:"id"`
	SessionID      string          `json:"session_id"`
	ParentThreadID string          `json:"parent_thread_id"`
	ForkedFromID   string          `json:"forked_from_id"`
	CWD            string          `json:"cwd"`
	Timestamp      string          `json:"timestamp"`
	AgentNickname  string          `json:"agent_nickname"`
	Source         json.RawMessage `json:"source"`
}

type rolloutSessionSource struct {
	Subagent *rolloutSubagentSource `json:"subagent"`
}

type rolloutSubagentSource struct {
	ThreadSpawn *rolloutThreadSpawn `json:"thread_spawn"`
}

type rolloutThreadSpawn struct {
	ParentThreadID string `json:"parent_thread_id"`
	AgentNickname  string `json:"agent_nickname"`
}

// discoverRolloutThreads scans session metadata into the narrow internal
// representation used for tree selection. Raw payloads never leave this call.
func discoverRolloutThreads(codexHome string) (rolloutCatalog, error) {
	paths, err := listRolloutPaths(codexHome)
	if err != nil {
		return rolloutCatalog{}, err
	}

	catalog := rolloutCatalog{Records: make([]ThreadRecord, 0, len(paths))}
	for _, path := range paths {
		record, malformed, identified, err := inspectRolloutMetadata(path)
		if err != nil {
			return rolloutCatalog{}, err
		}
		catalog.Diagnostics.FilesScanned++
		catalog.Diagnostics.MalformedLines += malformed
		if !identified {
			catalog.Diagnostics.UnidentifiedFiles++
			continue
		}
		catalog.Records = append(catalog.Records, record)
	}
	sort.Slice(catalog.Records, func(left, right int) bool {
		return catalog.Records[left].ID < catalog.Records[right].ID
	})
	return catalog, nil
}

func listRolloutPaths(codexHome string) ([]string, error) {
	paths := make([]string, 0)
	err := filepath.WalkDir(filepath.Join(codexHome, "sessions"), func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type().IsRegular() && strings.HasPrefix(entry.Name(), "rollout-") && strings.HasSuffix(entry.Name(), ".jsonl") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan Codex sessions: %w", err)
	}
	sort.Strings(paths)
	return paths, nil
}

func inspectRolloutMetadata(path string) (ThreadRecord, int, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return ThreadRecord{}, 0, false, fmt.Errorf("open rollout metadata: %w", err)
	}
	defer file.Close()

	var record ThreadRecord
	malformed := 0
	identified := false
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var envelope rolloutEnvelope
		if err := json.Unmarshal([]byte(line), &envelope); err != nil || envelope.Type == "" {
			malformed++
			continue
		}
		if timestamp, err := time.Parse(time.RFC3339Nano, envelope.Timestamp); err == nil && timestamp.After(record.LastActivityAt) {
			record.LastActivityAt = timestamp
		}
		if identified {
			reduceRolloutActivity(&record.Agent, json.RawMessage(line))
			continue
		}
		if envelope.Type != "session_meta" {
			continue
		}
		var metadata rolloutSessionMetadata
		if err := json.Unmarshal(envelope.Payload, &metadata); err != nil || !validThreadID(metadata.ID) {
			continue
		}
		record.ID = metadata.ID
		record.RolloutPath = path
		record.SessionID = metadata.SessionID
		record.ParentThreadID = metadata.ParentThreadID
		record.ForkedFromID = metadata.ForkedFromID
		record.CWD = metadata.CWD
		name := metadata.AgentNickname
		if spawn := metadata.threadSpawn(); spawn != nil {
			if record.ParentThreadID == "" {
				record.ParentThreadID = spawn.ParentThreadID
			}
			if name == "" {
				name = spawn.AgentNickname
			}
		}
		if name == "" {
			name = metadata.ID
		}
		role := "root"
		if record.ParentThreadID != "" {
			role = "subagent"
		}
		record.Agent = AgentNode{ID: metadata.ID, Name: name, Role: role, LifecycleState: "unknown", ActivityKind: "unknown", Presence: "idle"}
		if record.LastActivityAt.IsZero() {
			record.LastActivityAt, _ = time.Parse(time.RFC3339Nano, metadata.Timestamp)
		}
		identified = true
	}
	if err := scanner.Err(); err != nil {
		return ThreadRecord{}, malformed, false, fmt.Errorf("read rollout metadata: %w", err)
	}
	return record, malformed, identified, nil
}

func (metadata rolloutSessionMetadata) threadSpawn() *rolloutThreadSpawn {
	var source rolloutSessionSource
	if json.Unmarshal(metadata.Source, &source) != nil || source.Subagent == nil {
		return nil
	}
	return source.Subagent.ThreadSpawn
}

func validThreadID(id string) bool {
	return id != "" && strings.TrimSpace(id) == id
}
