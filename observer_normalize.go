package main

import "encoding/json"

type rolloutActivityPayload struct {
	Type string `json:"type"`
}

// reduceRolloutActivity deliberately decodes discriminators only. Free-form
// prompts, reasoning, commands, and tool output cannot enter AgentNode.
func reduceRolloutActivity(node *AgentNode, record json.RawMessage) bool {
	var envelope rolloutEnvelope
	if json.Unmarshal(record, &envelope) != nil {
		return false
	}
	if envelope.Type != "event_msg" && envelope.Type != "response_item" {
		return false
	}
	var payload rolloutActivityPayload
	if json.Unmarshal(envelope.Payload, &payload) != nil {
		return false
	}

	switch envelope.Type + "/" + payload.Type {
	case "event_msg/task_started", "event_msg/agent_reasoning", "response_item/reasoning":
		setAgentActivity(node, "running", "reasoning", "active")
	case "response_item/function_call", "response_item/custom_tool_call", "response_item/function_call_output", "response_item/custom_tool_call_output":
		setAgentActivity(node, "running", "tool", "active")
	case "response_item/web_search_call":
		setAgentActivity(node, "running", "research", "active")
	case "event_msg/task_complete":
		setAgentActivity(node, "completed", "unknown", "idle")
	case "event_msg/turn_aborted":
		setAgentActivity(node, "interrupted", "unknown", "idle")
	default:
		return false
	}
	return true
}

func setAgentActivity(node *AgentNode, lifecycle, activity, presence string) {
	node.LifecycleState = lifecycle
	node.ActivityKind = activity
	node.Presence = presence
}
