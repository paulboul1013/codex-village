package main

import "encoding/json"

type rolloutActivityPayload struct {
	Type   string          `json:"type"`
	Status string          `json:"status"`
	Action json.RawMessage `json:"action"`
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
		node.AttentionState = "none"
	case "event_msg/turn_aborted":
		setAgentActivity(node, "interrupted", "unknown", "idle")
		node.AttentionState = "none"
	case "event_msg/task_failed", "event_msg/turn_failed":
		setAgentActivity(node, "failed", "unknown", "idle")
		node.AttentionState = "none"
	case "event_msg/request_user_input", "event_msg/waiting_for_input":
		setAgentActivity(node, "waiting", "unknown", "idle")
		node.AttentionState = "waiting_for_input"
	case "event_msg/approval_request", "event_msg/waiting_for_approval":
		setAgentActivity(node, "waiting", "tool", "idle")
		node.AttentionState = "waiting_for_approval"
	case "event_msg/sub_agent_activity":
		return reduceDelegationActivity(node, payload)
	default:
		return false
	}
	return true
}

func setAgentActivity(node *AgentNode, lifecycle, activity, presence string) {
	node.LifecycleState = lifecycle
	node.ActivityKind = activity
	node.Presence = presence
	if lifecycle == "running" {
		node.AttentionState = "none"
	}
}

func reduceDelegationActivity(node *AgentNode, payload rolloutActivityPayload) bool {
	kind := payload.Status
	if kind == "" && len(payload.Action) > 0 {
		_ = json.Unmarshal(payload.Action, &kind)
	}
	switch kind {
	case "started", "spawned", "interacted":
		setAgentActivity(node, "running", "delegation", "active")
	case "waiting":
		setAgentActivity(node, "waiting", "delegation", "idle")
		node.AttentionState = "waiting_for_input"
	case "completed", "closed":
		setAgentActivity(node, "completed", "delegation", "idle")
		node.AttentionState = "none"
	case "interrupted":
		setAgentActivity(node, "interrupted", "delegation", "idle")
		node.AttentionState = "none"
	case "failed":
		setAgentActivity(node, "failed", "delegation", "idle")
		node.AttentionState = "none"
	default:
		return false
	}
	return true
}
