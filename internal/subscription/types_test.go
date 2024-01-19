package subscription

import (
	"testing"

	"github.com/nicklaw5/helix/v2"
	"github.com/stretchr/testify/assert"
)

func Test_formatCondition(t *testing.T) {
	tests := []struct {
		name string
		cond *helix.EventSubCondition
		want map[string]string
	}{
		{
			"empty struct yields empty map",
			&helix.EventSubCondition{},
			map[string]string{},
		},
		{
			"broadcast_user_id is conveyed",
			&helix.EventSubCondition{
				BroadcasterUserID: "1337",
			},
			map[string]string{
				"broadcaster_user_id": "1337",
			},
		},
		{
			"multiple fields are conveyed",
			&helix.EventSubCondition{
				ModeratorUserID:       "1337",
				FromBroadcasterUserID: "1337",
				ClientID:              "foobar",
			},
			map[string]string{
				"moderator_user_id":        "1337",
				"from_broadcaster_user_id": "1337",
				"client_id":                "foobar",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatCondition(tt.cond)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_parseCondition(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]string
		want helix.EventSubCondition
	}{
		{
			"empty map yields empty struc",
			map[string]string{},
			helix.EventSubCondition{},
		},
		{
			"broadcast_user_id is conveyed",
			map[string]string{
				"broadcaster_user_id": "1337",
			},
			helix.EventSubCondition{
				BroadcasterUserID: "1337",
			},
		},
		{
			"multiple fields are conveyed",
			map[string]string{
				"moderator_user_id":        "1337",
				"from_broadcaster_user_id": "1337",
				"client_id":                "foobar",
			},
			helix.EventSubCondition{
				ModeratorUserID:       "1337",
				FromBroadcasterUserID: "1337",
				ClientID:              "foobar",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCondition(tt.m)
			assert.Equal(t, tt.want, got)
		})
	}
}
