package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReduceRolloutActivityMapsAllowlistedLifecycleAndActivity(t *testing.T) {
	tests := []struct {
		name      string
		record    string
		lifecycle string
		activity  string
		presence  string
	}{
		{"turn starts", `{"type":"event_msg","payload":{"type":"task_started","turn_id":"private"}}`, "running", "reasoning", "active"},
		{"reasoning observed", `{"type":"response_item","payload":{"type":"reasoning","content":"private reasoning"}}`, "running", "reasoning", "active"},
		{"tool call observed", `{"type":"response_item","payload":{"type":"function_call","name":"shell","arguments":"private command"}}`, "running", "tool", "active"},
		{"web search observed", `{"type":"response_item","payload":{"type":"web_search_call","action":{"query":"private query"}}}`, "running", "research", "active"},
		{"turn completes", `{"type":"event_msg","payload":{"type":"task_complete","last_agent_message":"private answer"}}`, "completed", "unknown", "idle"},
		{"turn interrupted", `{"type":"event_msg","payload":{"type":"turn_aborted","reason":"private reason"}}`, "interrupted", "unknown", "idle"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			node := AgentNode{ID: "agent", LifecycleState: "unknown", ActivityKind: "unknown", Presence: "idle"}
			changed := reduceRolloutActivity(&node, json.RawMessage(test.record))
			if !changed {
				t.Fatal("record was not accepted")
			}
			if node.LifecycleState != test.lifecycle || node.ActivityKind != test.activity || node.Presence != test.presence {
				t.Fatalf("node = %+v, want lifecycle %q activity %q presence %q", node, test.lifecycle, test.activity, test.presence)
			}
			encoded, err := json.Marshal(node)
			if err != nil {
				t.Fatal(err)
			}
			for _, secret := range []string{"private reasoning", "private command", "private query", "private answer", "private reason", "shell"} {
				if strings.Contains(string(encoded), secret) {
					t.Fatalf("normalized node leaked %q: %s", secret, encoded)
				}
			}
		})
	}
}

func TestReduceRolloutActivityIgnoresUnknownAndUserContent(t *testing.T) {
	node := AgentNode{ID: "agent", LifecycleState: "completed", ActivityKind: "unknown", Presence: "idle"}
	for _, record := range []string{
		`{"type":"future_outer","payload":{"type":"task_started"}}`,
		`{"type":"event_msg","payload":{"type":"future_inner","content":"private"}}`,
		`{"type":"event_msg","payload":{"type":"user_message","message":"private prompt"}}`,
	} {
		if reduceRolloutActivity(&node, json.RawMessage(record)) {
			t.Fatalf("unsafe or unknown record changed node: %s", record)
		}
	}
	if node.LifecycleState != "completed" {
		t.Fatalf("ignored records changed node: %+v", node)
	}
}

func TestReduceRolloutActivityMapsObserverP0SignalsWithoutLeakingPayloads(t *testing.T) {
	tests := []struct {
		name, record, lifecycle, activity, attention string
	}{
		{"waiting input", `{"type":"event_msg","payload":{"type":"request_user_input","question":"private question"}}`, "waiting", "unknown", "waiting_for_input"},
		{"waiting approval", `{"type":"event_msg","payload":{"type":"approval_request","command":"private command"}}`, "waiting", "tool", "waiting_for_approval"},
		{"explicit failure", `{"type":"event_msg","payload":{"type":"turn_failed","error":"private error"}}`, "failed", "unknown", "none"},
		{"delegation started", `{"type":"event_msg","payload":{"type":"sub_agent_activity","status":"started","agent_id":"private-child"}}`, "running", "delegation", "none"},
		{"delegation waiting", `{"type":"event_msg","payload":{"type":"sub_agent_activity","action":"waiting","message":"private message"}}`, "waiting", "delegation", "waiting_for_input"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			node := AgentNode{ID: "agent", AttentionState: "none"}
			if !reduceRolloutActivity(&node, json.RawMessage(test.record)) {
				t.Fatal("record was not accepted")
			}
			if node.LifecycleState != test.lifecycle || node.ActivityKind != test.activity || node.AttentionState != test.attention {
				t.Fatalf("node = %+v", node)
			}
			encoded, err := json.Marshal(node)
			if err != nil || strings.Contains(string(encoded), "private") {
				t.Fatalf("safe node leaked input: %s (%v)", encoded, err)
			}
		})
	}
}
