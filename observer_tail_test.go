package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestJSONLTailStartsAtEOFAndReadsOnlyCompleteAppends(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	if err := os.WriteFile(path, []byte("{\"type\":\"historical\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	tail, err := openJSONLTail(path, true)
	if err != nil {
		t.Fatalf("open tail: %v", err)
	}
	if records, diagnostics, err := tail.poll(); err != nil || len(records) != 0 || diagnostics.MalformedLines != 0 {
		t.Fatalf("initial poll = records %d diagnostics %+v error %v, want empty", len(records), diagnostics, err)
	}

	appendFile(t, path, "{\"type\":\"partial\"")
	if records, _, err := tail.poll(); err != nil || len(records) != 0 {
		t.Fatalf("partial poll = records %d error %v, want empty", len(records), err)
	}
	appendFile(t, path, "}\n{\"type\":\"complete\"}\n")
	records, diagnostics, err := tail.poll()
	if err != nil {
		t.Fatalf("completed poll: %v", err)
	}
	if got := len(records); got != 2 {
		t.Fatalf("completed records = %d, want 2", got)
	}
	if diagnostics.MalformedLines != 0 {
		t.Fatalf("diagnostics = %+v, want no malformed lines", diagnostics)
	}
}

func TestJSONLTailSkipsMalformedCompleteLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	tail, err := openJSONLTail(path, false)
	if err != nil {
		t.Fatalf("open tail: %v", err)
	}
	appendFile(t, path, "not-json\n\n{\"type\":\"valid\"}\n")

	records, diagnostics, err := tail.poll()
	if err != nil {
		t.Fatalf("poll: %v", err)
	}
	if len(records) != 1 || diagnostics.MalformedLines != 1 {
		t.Fatalf("poll = records %d diagnostics %+v, want 1 valid and 1 malformed", len(records), diagnostics)
	}
	if records, diagnostics, err = tail.poll(); err != nil || len(records) != 0 || diagnostics.MalformedLines != 0 {
		t.Fatalf("repeat poll = records %d diagnostics %+v error %v, want no replay", len(records), diagnostics, err)
	}
}

func TestJSONLTailStartsNewGenerationAfterTruncation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollout.jsonl")
	if err := os.WriteFile(path, []byte("{\"type\":\"old\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tail, err := openJSONLTail(path, true)
	if err != nil {
		t.Fatalf("open tail: %v", err)
	}
	if err := os.WriteFile(path, []byte("{\"type\":\"new\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	records, diagnostics, err := tail.poll()
	if err != nil {
		t.Fatalf("poll truncated file: %v", err)
	}
	if len(records) != 1 || diagnostics.GenerationChanges != 1 || tail.generation != 2 {
		t.Fatalf("truncation = records %d diagnostics %+v generation %d, want rebuilt generation", len(records), diagnostics, tail.generation)
	}
}

func TestJSONLTailStartsNewGenerationAfterSamePathReplacement(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "rollout.jsonl")
	replacement := filepath.Join(dir, "replacement.jsonl")
	if err := os.WriteFile(path, []byte("{\"type\":\"old\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tail, err := openJSONLTail(path, true)
	if err != nil {
		t.Fatalf("open tail: %v", err)
	}
	if err := os.WriteFile(replacement, []byte("{\"type\":\"new\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(replacement, path); err != nil {
		t.Fatal(err)
	}

	records, diagnostics, err := tail.poll()
	if err != nil {
		t.Fatalf("poll replaced file: %v", err)
	}
	if len(records) != 1 || diagnostics.GenerationChanges != 1 || tail.generation != 2 {
		t.Fatalf("replacement = records %d diagnostics %+v generation %d, want rebuilt generation", len(records), diagnostics, tail.generation)
	}
}

func appendFile(t *testing.T, path, value string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := file.WriteString(value); err != nil {
		t.Fatal(err)
	}
}
