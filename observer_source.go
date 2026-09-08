package main

import (
	"fmt"
	"sync"
	"time"
)

type observerSource struct {
	mu                sync.RWMutex
	codexHome         string
	rootID            string
	records           []ThreadRecord
	world             normalizedWorld
	tails             map[string]*jsonlTail
	knownPaths        map[string]bool
	revision          uint64
	nextDiscovery     time.Time
	discoveryInterval time.Duration
}

func openObserverSource(codexHome string, selector ThreadSelector) (*observerSource, error) {
	catalog, err := discoverRolloutThreads(codexHome)
	if err != nil {
		return nil, err
	}
	source := &observerSource{
		codexHome:         codexHome,
		records:           catalog.Records,
		tails:             make(map[string]*jsonlTail),
		knownPaths:        make(map[string]bool),
		discoveryInterval: time.Second,
	}
	for _, record := range catalog.Records {
		source.knownPaths[record.RolloutPath] = true
	}
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
	source.rootID = root.ID
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

func (source *observerSource) discoverNewRollouts() (bool, error) {
	source.mu.Lock()
	defer source.mu.Unlock()
	return source.discoverNewRolloutsLocked()
}

func (source *observerSource) discoverNewRolloutsLocked() (bool, error) {
	paths, err := listRolloutPaths(source.codexHome)
	if err != nil {
		return false, err
	}
	added := false
	for _, path := range paths {
		if source.knownPaths[path] {
			continue
		}
		record, _, identified, err := inspectRolloutMetadata(path)
		if err != nil {
			return false, err
		}
		if !identified {
			continue
		}
		source.knownPaths[path] = true
		source.records = append(source.records, record)
		added = true
	}
	if !added || source.rootID == "" {
		return false, nil
	}

	tree, err := reconstructExecutionTree(source.records, source.rootID)
	if err != nil {
		return false, err
	}
	previous := make(map[string]AgentNode, len(source.world.Agents))
	for _, node := range source.world.Agents {
		previous[node.ID] = node
	}
	next := tree.normalizedWorld()
	worldChanged := len(next.Agents) != len(source.world.Agents)
	for index := range next.Agents {
		if node, exists := previous[next.Agents[index].ID]; exists {
			next.Agents[index] = node
		}
	}
	for _, thread := range tree.Threads {
		if source.tails[thread.ID] != nil || thread.RolloutPath == "" {
			continue
		}
		tail, err := openJSONLTail(thread.RolloutPath, true)
		if err != nil {
			return false, fmt.Errorf("tail new thread %s: %w", thread.ID, err)
		}
		source.tails[thread.ID] = tail
	}
	source.world = next
	if worldChanged {
		source.revision++
	}
	return worldChanged, nil
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
	now := time.Now()
	if source.nextDiscovery.IsZero() || !now.Before(source.nextDiscovery) {
		source.nextDiscovery = now.Add(source.discoveryInterval)
		discovered, err := source.discoverNewRolloutsLocked()
		if err != nil {
			return false, fmt.Errorf("discover live rollouts: %w", err)
		}
		changed = discovered
	}
	activityChanged := false
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
				activityChanged = true
			}
		}
	}
	if activityChanged {
		source.revision++
	}
	return changed || activityChanged, nil
}

func (source *observerSource) forceDiscoveryOnNextPoll() {
	source.mu.Lock()
	defer source.mu.Unlock()
	source.nextDiscovery = time.Time{}
}

func (source *observerSource) worldRevision() uint64 {
	source.mu.RLock()
	defer source.mu.RUnlock()
	return source.revision
}
