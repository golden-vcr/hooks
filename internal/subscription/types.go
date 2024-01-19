package subscription

import "github.com/nicklaw5/helix/v2"

// Status represents the status of all registered EventSub webhook subscriptions
type Status struct {
	Ok            bool    `json:"ok"`
	Subscriptions []State `json:"subscriptions"`
}

// State represents the state of a single EventSub subscription
type State struct {
	Required  bool              `json:"required"`
	Type      string            `json:"type"`
	Version   string            `json:"version"`
	Condition map[string]string `json:"condition"`
	Status    string            `json:"status"`

	subscriptionId string
}

// formatCondition converts a helix.EventSubCondition to a map[string]string so that it
// can be reliably JSON-serialized without including values for empty fields: the JSON
// struct tags on helix.EventSubCondition do not include omitempty; so this is a simple
// workaround
func formatCondition(cond *helix.EventSubCondition) map[string]string {
	result := make(map[string]string)
	if cond.BroadcasterUserID != "" {
		result["broadcaster_user_id"] = cond.BroadcasterUserID
	}
	if cond.FromBroadcasterUserID != "" {
		result["from_broadcaster_user_id"] = cond.FromBroadcasterUserID
	}
	if cond.ModeratorUserID != "" {
		result["moderator_user_id"] = cond.ModeratorUserID
	}
	if cond.ToBroadcasterUserID != "" {
		result["to_broadcaster_user_id"] = cond.ToBroadcasterUserID
	}
	if cond.RewardID != "" {
		result["reward_id"] = cond.RewardID
	}
	if cond.ClientID != "" {
		result["client_id"] = cond.ClientID
	}
	if cond.ExtensionClientID != "" {
		result["extension_client_id"] = cond.ExtensionClientID
	}
	if cond.UserID != "" {
		result["user_id"] = cond.UserID
	}
	return result
}

// parseCondition converts from a map[string]string back to an equivalent
// helix.EventSubCondition struct
func parseCondition(m map[string]string) helix.EventSubCondition {
	return helix.EventSubCondition{
		BroadcasterUserID:     m["broadcaster_user_id"],
		FromBroadcasterUserID: m["from_broadcaster_user_id"],
		ModeratorUserID:       m["moderator_user_id"],
		ToBroadcasterUserID:   m["to_broadcaster_user_id"],
		RewardID:              m["reward_id"],
		ClientID:              m["client_id"],
		ExtensionClientID:     m["extension_client_id"],
		UserID:                m["user_id"],
	}
}
