package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"syscall"
	"time"
)

type fileIdentity struct {
	device uint64
	inode  uint64
}

type jsonlTailDiagnostics struct {
	MalformedLines    int
	GenerationChanges int
}

// jsonlTail reads rollout files without ever modifying them. Offset advances
// only through newline-terminated records, so a partial final line is retried.
type jsonlTail struct {
	path         string
	identity     fileIdentity
	offset       int64
	observedSize int64
	modifiedAt   time.Time
	generation   uint64
}

func openJSONLTail(path string, startAtEOF bool) (*jsonlTail, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat rollout: %w", err)
	}
	identity, err := identityFromFileInfo(info)
	if err != nil {
		return nil, err
	}
	offset := int64(0)
	if startAtEOF {
		offset = info.Size()
	}
	return &jsonlTail{
		path: path, identity: identity, offset: offset,
		observedSize: info.Size(), modifiedAt: info.ModTime(), generation: 1,
	}, nil
}

func (tail *jsonlTail) poll() ([]json.RawMessage, jsonlTailDiagnostics, error) {
	file, err := os.Open(tail.path)
	if err != nil {
		return nil, jsonlTailDiagnostics{}, fmt.Errorf("open rollout: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, jsonlTailDiagnostics{}, fmt.Errorf("stat open rollout: %w", err)
	}
	identity, err := identityFromFileInfo(info)
	if err != nil {
		return nil, jsonlTailDiagnostics{}, err
	}
	diagnostics := jsonlTailDiagnostics{}
	if identity != tail.identity || info.Size() < tail.offset ||
		(info.Size() <= tail.offset && info.Size() == tail.observedSize && !info.ModTime().Equal(tail.modifiedAt)) {
		tail.identity = identity
		tail.offset = 0
		tail.generation++
		diagnostics.GenerationChanges++
	}

	if _, err := file.Seek(tail.offset, io.SeekStart); err != nil {
		return nil, diagnostics, fmt.Errorf("seek rollout: %w", err)
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, diagnostics, fmt.Errorf("read rollout: %w", err)
	}

	records := make([]json.RawMessage, 0)
	reader := bufio.NewReader(bytes.NewReader(data))
	for {
		line, readErr := reader.ReadBytes('\n')
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, diagnostics, fmt.Errorf("split rollout: %w", readErr)
		}
		tail.offset += int64(len(line))
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var object map[string]json.RawMessage
		if json.Unmarshal(line, &object) != nil || object == nil {
			diagnostics.MalformedLines++
			continue
		}
		records = append(records, json.RawMessage(bytes.Clone(line)))
	}

	tail.observedSize = info.Size()
	tail.modifiedAt = info.ModTime()
	return records, diagnostics, nil
}

func identityFromFileInfo(info os.FileInfo) (fileIdentity, error) {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fileIdentity{}, fmt.Errorf("rollout file identity unavailable")
	}
	return fileIdentity{device: uint64(stat.Dev), inode: stat.Ino}, nil
}
