package main

type observerSource struct {
	records []ThreadRecord
	world   normalizedWorld
}

func openObserverSource(codexHome string, selector ThreadSelector) (observerSource, error) {
	catalog, err := discoverRolloutThreads(codexHome)
	if err != nil {
		return observerSource{}, err
	}
	source := observerSource{records: catalog.Records}
	if selector.ThreadID == "" && !selector.Latest {
		return source, nil
	}
	root, err := selectExecutionRoot(catalog.Records, selector)
	if err != nil {
		return observerSource{}, err
	}
	tree, err := reconstructExecutionTree(catalog.Records, root.ID)
	if err != nil {
		return observerSource{}, err
	}
	source.world = tree.normalizedWorld()
	return source, nil
}

func (source observerSource) threadRecords() []ThreadRecord {
	return source.records
}

func (source observerSource) normalizedWorld() normalizedWorld {
	return source.world
}
