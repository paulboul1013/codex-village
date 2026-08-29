package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

func TestDemoServerServesHealthAndCanvas(t *testing.T) {
	server := httptest.NewServer(newServer())
	t.Cleanup(server.Close)

	health, err := http.Get(server.URL + "/api/health")
	if err != nil {
		t.Fatalf("GET health: %v", err)
	}
	t.Cleanup(func() { _ = health.Body.Close() })

	if health.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d, want %d", health.StatusCode, http.StatusOK)
	}
	if contentType := health.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		t.Fatalf("health content type = %q, want JSON", contentType)
	}

	page, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("GET demo page: %v", err)
	}
	t.Cleanup(func() { _ = page.Body.Close() })

	if page.StatusCode != http.StatusOK {
		t.Fatalf("page status = %d, want %d", page.StatusCode, http.StatusOK)
	}
	if contentType := page.Header.Get("Content-Type"); !strings.HasPrefix(contentType, "text/html") {
		t.Fatalf("page content type = %q, want HTML", contentType)
	}
	pageBody, err := io.ReadAll(page.Body)
	if err != nil {
		t.Fatalf("read demo page: %v", err)
	}
	if !strings.Contains(string(pageBody), "<canvas") {
		t.Fatalf("demo page does not contain a Canvas: %s", pageBody)
	}
}

func TestDemoServerStreamsInitialSafeWorldSnapshot(t *testing.T) {
	server := httptest.NewServer(newServer())
	t.Cleanup(server.Close)

	websocketURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	connection, response, err := websocket.Dial(context.Background(), websocketURL, nil)
	if err != nil {
		if response != nil {
			t.Fatalf("dial demo WebSocket: %v (status %s)", err, response.Status)
		}
		t.Fatalf("dial demo WebSocket: %v", err)
	}
	t.Cleanup(func() { _ = connection.Close(websocket.StatusNormalClosure, "") })

	var snapshot WorldSnapshot
	if err := wsjson.Read(context.Background(), connection, &snapshot); err != nil {
		t.Fatalf("read demo snapshot: %v", err)
	}

	if snapshot.Type != "snapshot" {
		t.Fatalf("snapshot type = %q, want snapshot", snapshot.Type)
	}
	if len(snapshot.Agents) != 2 {
		t.Fatalf("agent count = %d, want 2", len(snapshot.Agents))
	}
	if snapshot.Agents[0].Role != "root" || snapshot.Agents[0].ParentID != "" {
		t.Fatalf("root = %+v, want root without parent", snapshot.Agents[0])
	}
	if snapshot.Agents[1].ParentID != snapshot.Agents[0].ID {
		t.Fatalf("subagent parent = %q, want %q", snapshot.Agents[1].ParentID, snapshot.Agents[0].ID)
	}
}

func TestDemoSnapshotUsesNormalizedWorldContract(t *testing.T) {
	snapshot := demoSnapshot()

	if snapshot.Type != "snapshot" {
		t.Fatalf("snapshot type = %q, want snapshot", snapshot.Type)
	}
	if len(snapshot.Agents) != 2 {
		t.Fatalf("agent count = %d, want 2", len(snapshot.Agents))
	}
	if snapshot.Agents[0].Role != "root" || snapshot.Agents[0].ParentID != "" {
		t.Fatalf("root agent = %+v, want root without parent", snapshot.Agents[0])
	}
	if snapshot.Agents[1].ParentID != snapshot.Agents[0].ID {
		t.Fatalf("subagent parent = %q, want %q", snapshot.Agents[1].ParentID, snapshot.Agents[0].ID)
	}
}

func TestDemoSourceFlowsThroughNormalizedWorldSnapshot(t *testing.T) {
	source := demoSource{}
	snapshot := worldSnapshot(source)

	if snapshot.Type != "snapshot" {
		t.Fatalf("snapshot type = %q, want snapshot", snapshot.Type)
	}
	if len(snapshot.Agents) != 2 {
		t.Fatalf("agent count = %d, want 2", len(snapshot.Agents))
	}
	if snapshot.Agents[1].ParentID != snapshot.Agents[0].ID {
		t.Fatalf("subagent parent = %q, want %q", snapshot.Agents[1].ParentID, snapshot.Agents[0].ID)
	}
}

func TestDemoSnapshotSerializesOnlySafeFields(t *testing.T) {
	encoded, err := json.Marshal(demoSnapshot())
	if err != nil {
		t.Fatalf("marshal demo snapshot: %v", err)
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("decode demo snapshot: %v", err)
	}
	for field := range payload {
		if field != "type" && field != "agents" {
			t.Fatalf("unexpected snapshot field %q", field)
		}
	}

	var agents []map[string]json.RawMessage
	if err := json.Unmarshal(payload["agents"], &agents); err != nil {
		t.Fatalf("decode agents: %v", err)
	}
	allowedAgentFields := map[string]bool{
		"id": true, "parentId": true, "name": true, "role": true,
		"lifecycleState": true, "activityKind": true, "presence": true,
	}
	for _, agent := range agents {
		for field := range agent {
			if !allowedAgentFields[field] {
				t.Fatalf("unexpected agent field %q", field)
			}
		}
	}
}

func TestDemoServerKeepsWebSocketOpenAfterInitialSnapshot(t *testing.T) {
	server := httptest.NewServer(newServer())
	t.Cleanup(server.Close)

	websocketURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws"
	connection, _, err := websocket.Dial(context.Background(), websocketURL, nil)
	if err != nil {
		t.Fatalf("dial demo WebSocket: %v", err)
	}
	t.Cleanup(func() { _ = connection.Close(websocket.StatusNormalClosure, "") })

	var snapshot map[string]any
	if err := wsjson.Read(context.Background(), connection, &snapshot); err != nil {
		t.Fatalf("read demo snapshot: %v", err)
	}

	connection.CloseRead(context.Background())
	pingContext, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := connection.Ping(pingContext); err != nil {
		t.Fatalf("ping after snapshot = %v, want open connection", err)
	}
}
