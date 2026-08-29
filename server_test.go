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

func TestDemoServerSelectsSafeTreeByHTTP(t *testing.T) {
	server := httptest.NewServer(newServer())
	t.Cleanup(server.Close)

	response, err := http.Get(server.URL + "/api/tree?thread=scout-demo")
	if err != nil {
		t.Fatalf("GET selected tree: %v", err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	if response.StatusCode != http.StatusOK {
		t.Fatalf("selected tree status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	var snapshot WorldSnapshot
	if err := json.NewDecoder(response.Body).Decode(&snapshot); err != nil {
		t.Fatalf("decode selected tree: %v", err)
	}
	if got := len(snapshot.Agents); got != 1 {
		t.Fatalf("selected child agent count = %d, want 1", got)
	}
	if snapshot.Agents[0].ID != "scout-demo" || snapshot.Agents[0].ParentID != "" {
		t.Fatalf("selected child agent = %+v, want child view root", snapshot.Agents[0])
	}
}

func TestDemoServerUsesCWDAsSafeRootPickerFilter(t *testing.T) {
	server := httptest.NewServer(newServer())
	t.Cleanup(server.Close)

	response, err := http.Get(server.URL + "/api/tree?cwd=/demo/.")
	if err != nil {
		t.Fatalf("GET root picker: %v", err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	if response.StatusCode != http.StatusOK {
		t.Fatalf("root picker status = %d, want %d", response.StatusCode, http.StatusOK)
	}

	var picker rootPickerResponse
	if err := json.NewDecoder(response.Body).Decode(&picker); err != nil {
		t.Fatalf("decode root picker: %v", err)
	}
	if picker.Type != "root_picker" || len(picker.Roots) != 1 || picker.Roots[0].ID != "root-demo" {
		t.Fatalf("root picker = %+v, want only root-demo", picker)
	}
}

func TestDemoServerReturnsSafeSelectionError(t *testing.T) {
	server := httptest.NewServer(newServer())
	t.Cleanup(server.Close)

	response, err := http.Get(server.URL + "/api/tree?thread=missing")
	if err != nil {
		t.Fatalf("GET missing tree: %v", err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("missing tree status = %d, want %d", response.StatusCode, http.StatusNotFound)
	}

	var failure selectionErrorResponse
	if err := json.NewDecoder(response.Body).Decode(&failure); err != nil {
		t.Fatalf("decode selection error: %v", err)
	}
	if failure.Error != "thread_not_found" {
		t.Fatalf("selection error = %q, want thread_not_found", failure.Error)
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
