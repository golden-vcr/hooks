package hooks

import (
	"testing"

	"github.com/nicklaw5/helix/v2"
	"github.com/stretchr/testify/assert"
)

func Test_RequiredSubscriptionConditionParams_Format(t *testing.T) {
	params := &RequiredSubscriptionConditionParams{
		ChannelUserId: "1337",
	}
	got, err := params.Format(&helix.EventSubCondition{
		UserID:   "{{.ChannelUserId}}",
		RewardID: "channel-{{.ChannelUserId}}-reward",
	})
	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, &helix.EventSubCondition{
		UserID:   "1337",
		RewardID: "channel-1337-reward",
	}, got)
}

func Test_GetRequiredUserScopes(t *testing.T) {
	required := RequiredSubscriptions{
		{
			RequiredScopes: []string{
				"moderator:read:followers",
			},
		},
		{
			RequiredScopes: []string{
				"moderator:read:followers",
				"user:read:follows",
			},
		},
		{},
		{
			RequiredScopes: []string{
				"moderator:read:followers",
				"user:read:subscriptions",
			},
		},
	}
	got := required.GetRequiredUserScopes()
	assert.ElementsMatch(t, got, []string{
		"moderator:read:followers",
		"user:read:follows",
		"user:read:subscriptions",
	})
}
