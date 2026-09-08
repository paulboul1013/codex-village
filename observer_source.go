package main

import (
	"fmt"
	"sync"
)

type observerSource struct {
	mu       sync.RWMutex
	records  []ThreadRecord
	world    normalizedWorld
	tails    map[string]*jsonlTail
	revision uint64
}

func openObserverSource(codexHome string, selector ThreadSelector) (*observerSource, error) {
	catalog, err := discoverRolloutThreads(codexHome)
	if err != nil {
		return nil, err
	}
	source := &observerSource{records: catalog.Records, tails: make(map[string]*jsonlTail)}
	if selector.ThreadID == "" && !selector.Latest {
		return source, nil
	}
	root, err := selectExecutionRoot(catalog.Records, selector)
	if err != nil {
		return nil, err
	}
	tree, err := reconstructExecutionTree(catalog.Records, root.ID)
	if err != nil {
		return nil, err
	}
	source.world = tree.normalizedWorld()
	for _, thread := range tree.Threads {
		if thread.RolloutPath == "" {
			continue
		}
		tail, err := openJSONLTail(thread.RolloutPath, true)
		if err != nil {
			return nil, fmt.Errorf("tail thread %s: %w", thread.ID, err)
		}
		source.tails[thread.ID] = tail
	}
	return source, nil
}

func (source *observerSource) threadRecords() []ThreadRecord {
	source.mu.RLock()
	defer source.mu.RUnlock()
	return append([]ThreadRecord(nil), source.records...)
}

func (source *observerSource) normalizedWorld() normalizedWorld {
	source.mu.RLock()
	defer source.mu.RUnlock()
	return normalizedWorld{Agents: append([]AgentNode(nil), source.world.Agents...)}
}

func (source *observerSource) poll() (bool, error) {
	source.mu.Lock()
	defer source.mu.Unlock()

	changed := false
	for index := range source.world.Agents {
		node := &source.world.Agents[index]
		tail := source.tails[node.ID]
		if tail == nil {
			continue
		}
		records, _, err := tail.poll()
		if err != nil {
			return changed, fmt.Errorf("poll thread %s: %w", node.ID, err)
		}
		for _, record := range records {
			if reduceRolloutActivity(node, record) {
				changed = true
			}
		}
	}
	if changed {
		source.revision++
	}
	return changed, nil
}

func (source *observerSource) worldRevision() uint64 {
	source.mu.RLock()
	defer source.mu.RUnlock()
	return source.revision
}
