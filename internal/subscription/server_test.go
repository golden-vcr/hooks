package subscription

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/golden-vcr/hooks"
	"github.com/nicklaw5/helix/v2"
	"github.com/stretchr/testify/assert"
)

func Test_Server_handleGetSubscriptions(t *testing.T) {
	tests := []struct {
		name       string
		required   hooks.RequiredSubscriptions
		c          *mockTwitchClient
		wantStatus int
		wantBody   string
	}{
		{
			"nothing required, nothing registered",
			hooks.RequiredSubscriptions{},
			&mockTwitchClient{},
			200,
			`{"ok":true,"subscriptions":[]}`,
		},
		{
			"one subscription required; nothing registered",
			hooks.RequiredSubscriptions{
				{
					Type:    helix.EventSubTypeChannelUpdate,
					Version: "2",
					TemplatedCondition: helix.EventSubCondition{
						BroadcasterUserID: "{{.ChannelUserId}}",
					},
				},
			},
			&mockTwitchClient{},
			200,
			`{"ok":false,"subscriptions":[{"required":true,"type":"channel.update","version":"2","condition":{"broadcaster_user_id":"1337"},"status":"missing"}]}`,
		},
		{
			"one subscription required; one matching subscription registered",
			hooks.RequiredSubscriptions{
				{
					Type:    helix.EventSubTypeChannelUpdate,
					Version: "2",
					TemplatedCondition: helix.EventSubCondition{
						BroadcasterUserID: "{{.ChannelUserId}}",
					},
				},
			},
			&mockTwitchClient{
				subscriptions: []helix.EventSubSubscription{
					{
						ID:      "10000001",
						Type:    helix.EventSubTypeChannelUpdate,
						Version: "2",
						Condition: helix.EventSubCondition{
							BroadcasterUserID: "1337",
						},
						Transport: helix.EventSubTransport{
							Method:   "webhook",
							Callback: "https://my-cool-service.com/callback",
							Secret:   "my-cool-webhook-secret",
						},
						Status: "enabled",
					},
				},
			},
			200,
			`{"ok":true,"subscriptions":[{"required":true,"type":"channel.update","version":"2","condition":{"broadcaster_user_id":"1337"},"status":"enabled"}]}`,
		},
		{
			"if matching subscription is not enabled, overall status is not ok",
			hooks.RequiredSubscriptions{
				{
					Type:    helix.EventSubTypeChannelUpdate,
					Version: "2",
					TemplatedCondition: helix.EventSubCondition{
						BroadcasterUserID: "{{.ChannelUserId}}",
					},
				},
			},
			&mockTwitchClient{
				subscriptions: []helix.EventSubSubscription{
					{
						ID:      "10000001",
						Type:    helix.EventSubTypeChannelUpdate,
						Version: "2",
						Condition: helix.EventSubCondition{
							BroadcasterUserID: "1337",
						},
						Transport: helix.EventSubTransport{
							Method:   "webhook",
							Callback: "https://my-cool-service.com/callback",
							Secret:   "my-cool-webhook-secret",
						},
						Status: "webhook_callback_verification_failed",
					},
				},
			},
			200,
			`{"ok":false,"subscriptions":[{"required":true,"type":"channel.update","version":"2","condition":{"broadcaster_user_id":"1337"},"status":"webhook_callback_verification_failed"}]}`,
		},
		{
			"one required subscription unsatisfied; one irrelevant subscription registered",
			hooks.RequiredSubscriptions{
				{
					Type:    helix.EventSubTypeChannelSubscriptionGift,
					Version: "1",
					TemplatedCondition: helix.EventSubCondition{
						BroadcasterUserID: "{{.ChannelUserId}}",
					},
				},
			},
			&mockTwitchClient{
				subscriptions: []helix.EventSubSubscription{
					{
						ID:      "10000001",
						Type:    helix.EventSubTypeChannelUpdate,
						Version: "2",
						Condition: helix.EventSubCondition{
							BroadcasterUserID: "1337",
						},
						Transport: helix.EventSubTransport{
							Method:   "webhook",
							Callback: "https://my-cool-service.com/callback",
							Secret:   "my-cool-webhook-secret",
						},
						Status: "enabled",
					},
				},
			},
			200,
			`{"ok":false,"subscriptions":[{"required":true,"type":"channel.subscription.gift","version":"1","condition":{"broadcaster_user_id":"1337"},"status":"missing"},{"required":false,"type":"channel.update","version":"2","condition":{"broadcaster_user_id":"1337"},"status":"enabled"}]}`,
		},
		{
			"existing subscriptions not matching channel user ID are entirely ignored",
			hooks.RequiredSubscriptions{
				{
					Type:    helix.EventSubTypeChannelUpdate,
					Version: "2",
					TemplatedCondition: helix.EventSubCondition{
						BroadcasterUserID: "{{.ChannelUserId}}",
					},
				},
			},
			&mockTwitchClient{
				subscriptions: []helix.EventSubSubscription{
					{
						ID:      "10000001",
						Type:    helix.EventSubTypeChannelUpdate,
						Version: "2",
						Condition: helix.EventSubCondition{
							BroadcasterUserID: "9999",
						},
						Transport: helix.EventSubTransport{
							Method:   "webhook",
							Callback: "https://my-cool-service.com/callback",
							Secret:   "my-cool-webhook-secret",
						},
						Status: "enabled",
					},
				},
			},
			200,
			`{"ok":false,"subscriptions":[{"required":true,"type":"channel.update","version":"2","condition":{"broadcaster_user_id":"1337"},"status":"missing"}]}`,
		},
		{
			"existing subscriptions not matching callback URL are entirely ignored",
			hooks.RequiredSubscriptions{
				{
					Type:    helix.EventSubTypeChannelUpdate,
					Version: "2",
					TemplatedCondition: helix.EventSubCondition{
						BroadcasterUserID: "{{.ChannelUserId}}",
					},
				},
			},
			&mockTwitchClient{
				subscriptions: []helix.EventSubSubscription{
					{
						ID:      "10000001",
						Type:    helix.EventSubTypeChannelUpdate,
						Version: "2",
						Condition: helix.EventSubCondition{
							BroadcasterUserID: "1337",
						},
						Transport: helix.EventSubTransport{
							Method:   "webhook",
							Callback: "https://a-different-url.com/callback",
							Secret:   "my-cool-webhook-secret",
						},
						Status: "enabled",
					},
				},
			},
			200,
			`{"ok":false,"subscriptions":[{"required":true,"type":"channel.update","version":"2","condition":{"broadcaster_user_id":"1337"},"status":"missing"}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Server{
				callbackUrl: "https://my-cool-service.com/callback",
				conditionParams: hooks.RequiredSubscriptionConditionParams{
					ChannelUserId: "1337",
				},
				requiredSubscriptions: tt.required,
				newTwitchClient: func(ctx context.Context) (TwitchClient, error) {
					return tt.c, nil
				},
				twitchWebhookSecret: "my-cool-webhook-secret",
			}
			req := httptest.NewRequest(http.MethodGet, "/subscriptions", nil)
			res := httptest.NewRecorder()
			s.handleGetSubscriptions(res, req)

			b, err := io.ReadAll(res.Body)
			assert.NoError(t, err)
			body := strings.TrimSuffix(string(b), "\n")
			assert.Equal(t, tt.wantStatus, res.Code)
			assert.Equal(t, tt.wantBody, body)
		})
	}
}

func Test_Server_handlePatchSubscriptions(t *testing.T) {
	tests := []struct {
		name                string
		required            hooks.RequiredSubscriptions
		c                   *mockTwitchClient
		wantStatus          int
		wantBody            string
		wantSubscriptionIds []string
	}{
		{
			"nothing required, no changes",
			hooks.RequiredSubscriptions{},
			&mockTwitchClient{},
			204,
			"",
			[]string{},
		},
		{
			"one subscription required and not present; will be created",
			hooks.RequiredSubscriptions{
				{
					Type:    helix.EventSubTypeChannelUpdate,
					Version: "2",
					TemplatedCondition: helix.EventSubCondition{
						BroadcasterUserID: "{{.ChannelUserId}}",
					},
				},
			},
			&mockTwitchClient{},
			204,
			"",
			[]string{"10000001"},
		},
		{
			"existing subscription in enabled state will remain as is and not be replaced",
			hooks.RequiredSubscriptions{
				{
					Type:    helix.EventSubTypeChannelUpdate,
					Version: "2",
					TemplatedCondition: helix.EventSubCondition{
						BroadcasterUserID: "{{.ChannelUserId}}",
					},
				},
			},
			&mockTwitchClient{
				subscriptions: []helix.EventSubSubscription{
					{
						ID:      "10000001",
						Type:    helix.EventSubTypeChannelUpdate,
						Version: "2",
						Condition: helix.EventSubCondition{
							BroadcasterUserID: "1337",
						},
						Transport: helix.EventSubTransport{
							Method:   "webhook",
							Callback: "https://my-cool-service.com/callback",
							Secret:   "my-cool-webhook-secret",
						},
						Status: "enabled",
					},
				},
			},
			204,
			"",
			[]string{"10000001"},
		},
		{
			"existing subscription in non-enabled state will remain as is and not be replaced",
			hooks.RequiredSubscriptions{
				{
					Type:    helix.EventSubTypeChannelUpdate,
					Version: "2",
					TemplatedCondition: helix.EventSubCondition{
						BroadcasterUserID: "{{.ChannelUserId}}",
					},
				},
			},
			&mockTwitchClient{
				subscriptions: []helix.EventSubSubscription{
					{
						ID:      "10000001",
						Type:    helix.EventSubTypeChannelUpdate,
						Version: "2",
						Condition: helix.EventSubCondition{
							BroadcasterUserID: "1337",
						},
						Transport: helix.EventSubTransport{
							Method:   "webhook",
							Callback: "https://my-cool-service.com/callback",
							Secret:   "my-cool-webhook-secret",
						},
						Status: "webhook_callback_verification_failed ",
					},
				},
			},
			204,
			"",
			[]string{"10000001"},
		},
		{
			"irrelevant subscriptions are ignored",
			hooks.RequiredSubscriptions{
				{
					Type:    helix.EventSubTypeChannelUpdate,
					Version: "2",
					TemplatedCondition: helix.EventSubCondition{
						BroadcasterUserID: "{{.ChannelUserId}}",
					},
				},
			},
			&mockTwitchClient{
				subscriptions: []helix.EventSubSubscription{
					{
						ID:      "10000001",
						Type:    helix.EventSubTypeChannelUpdate,
						Version: "2",
						Condition: helix.EventSubCondition{
							BroadcasterUserID: "1337",
						},
						Transport: helix.EventSubTransport{
							Method:   "webhook",
							Callback: "https://an-entirely-different-url.com/callback",
							Secret:   "my-cool-webhook-secret",
						},
						Status: "enabled",
					},
				},
			},
			204,
			"",
			[]string{"10000001", "10000002"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Server{
				callbackUrl: "https://my-cool-service.com/callback",
				conditionParams: hooks.RequiredSubscriptionConditionParams{
					ChannelUserId: "1337",
				},
				requiredSubscriptions: tt.required,
				newTwitchClient: func(ctx context.Context) (TwitchClient, error) {
					return tt.c, nil
				},
				twitchWebhookSecret: "my-cool-webhook-secret",
			}
			req := httptest.NewRequest(http.MethodPatch, "/subscriptions", nil)
			res := httptest.NewRecorder()
			s.handlePatchSubscriptions(res, req)

			b, err := io.ReadAll(res.Body)
			assert.NoError(t, err)
			body := strings.TrimSuffix(string(b), "\n")
			assert.Equal(t, tt.wantStatus, res.Code)
			assert.Equal(t, tt.wantBody, body)

			subscriptionIds := make([]string, 0)
			for _, subscription := range tt.c.subscriptions {
				subscriptionIds = append(subscriptionIds, subscription.ID)
			}
			assert.ElementsMatch(t, subscriptionIds, tt.wantSubscriptionIds)
		})
	}
}

func Test_Server_handleDeleteSubscriptions(t *testing.T) {
	tests := []struct {
		name                         string
		required                     hooks.RequiredSubscriptions
		c                            *mockTwitchClient
		wantStatus                   int
		wantBody                     string
		wantRemainingSubscriptionIds []string
	}{
		{
			"nothing required, nothing registered, no result",
			hooks.RequiredSubscriptions{},
			&mockTwitchClient{},
			204,
			"",
			[]string{},
		},
		{
			"one required, two registered, both deleted",
			hooks.RequiredSubscriptions{
				{
					Type:    helix.EventSubTypeChannelUpdate,
					Version: "2",
					TemplatedCondition: helix.EventSubCondition{
						BroadcasterUserID: "{{.ChannelUserId}}",
					},
				},
			},
			&mockTwitchClient{
				subscriptions: []helix.EventSubSubscription{
					{
						ID:      "10000001",
						Type:    helix.EventSubTypeChannelUpdate,
						Version: "2",
						Condition: helix.EventSubCondition{
							BroadcasterUserID: "1337",
						},
						Transport: helix.EventSubTransport{
							Method:   "webhook",
							Callback: "https://my-cool-service.com/callback",
							Secret:   "my-cool-webhook-secret",
						},
						Status: "enabled",
					},
					{
						ID:      "10000002",
						Type:    helix.EventSubTypeChannelSubscriptionGift,
						Version: "1",
						Condition: helix.EventSubCondition{
							BroadcasterUserID: "1337",
						},
						Transport: helix.EventSubTransport{
							Method:   "webhook",
							Callback: "https://my-cool-service.com/callback",
							Secret:   "my-cool-webhook-secret",
						},
						Status: "enabled",
					},
				},
			},
			204,
			"",
			[]string{},
		},
		{
			"existing subscriptions with mismatched user ID or callback URL remain untouched",
			hooks.RequiredSubscriptions{},
			&mockTwitchClient{
				subscriptions: []helix.EventSubSubscription{
					{
						ID:      "10000001",
						Type:    helix.EventSubTypeChannelUpdate,
						Version: "2",
						Condition: helix.EventSubCondition{
							BroadcasterUserID: "1337",
						},
						Transport: helix.EventSubTransport{
							Method:   "webhook",
							Callback: "https://my-cool-service.com/callback",
							Secret:   "my-cool-webhook-secret",
						},
						Status: "enabled",
					},
					{
						ID:      "10000002",
						Type:    helix.EventSubTypeChannelSubscriptionGift,
						Version: "1",
						Condition: helix.EventSubCondition{
							BroadcasterUserID: "9999",
						},
						Transport: helix.EventSubTransport{
							Method:   "webhook",
							Callback: "https://my-cool-service.com/callback",
							Secret:   "my-cool-webhook-secret",
						},
						Status: "enabled",
					},
					{
						ID:      "10000003",
						Type:    helix.EventSubTypeChannelSubscription,
						Version: "1",
						Condition: helix.EventSubCondition{
							BroadcasterUserID: "1337",
						},
						Transport: helix.EventSubTransport{
							Method:   "webhook",
							Callback: "https://an-entirely-different-url.com/callback",
							Secret:   "my-cool-webhook-secret",
						},
						Status: "enabled",
					},
				},
			},
			204,
			"",
			[]string{"10000002", "10000003"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Server{
				callbackUrl: "https://my-cool-service.com/callback",
				conditionParams: hooks.RequiredSubscriptionConditionParams{
					ChannelUserId: "1337",
				},
				requiredSubscriptions: tt.required,
				newTwitchClient: func(ctx context.Context) (TwitchClient, error) {
					return tt.c, nil
				},
				twitchWebhookSecret: "my-cool-webhook-secret",
			}
			req := httptest.NewRequest(http.MethodDelete, "/subscriptions", nil)
			res := httptest.NewRecorder()
			s.handleDeleteSubscriptions(res, req)

			b, err := io.ReadAll(res.Body)
			assert.NoError(t, err)
			body := strings.TrimSuffix(string(b), "\n")
			assert.Equal(t, tt.wantStatus, res.Code)
			assert.Equal(t, tt.wantBody, body)

			remainingSubscriptionIds := make([]string, 0)
			for _, subscription := range tt.c.subscriptions {
				remainingSubscriptionIds = append(remainingSubscriptionIds, subscription.ID)
			}
			assert.ElementsMatch(t, remainingSubscriptionIds, tt.wantRemainingSubscriptionIds)
		})
	}
}

type mockTwitchClient struct {
	subscriptions []helix.EventSubSubscription
}

func (m *mockTwitchClient) GetEventSubSubscriptions(params *helix.EventSubSubscriptionsParams) (*helix.EventSubSubscriptionsResponse, error) {
	if params.After != "" {
		return nil, fmt.Errorf("pagination not mocked")
	}
	if params.Status != "" {
		return nil, fmt.Errorf("filtering by status not nocked")
	}
	if params.Type != "" {
		return nil, fmt.Errorf("filtering by type not mocked")
	}

	matches := make([]helix.EventSubSubscription, 0, len(m.subscriptions))
	for _, subscription := range m.subscriptions {
		if params.UserID == "" || matchesUserId(&subscription, params.UserID) {
			matches = append(matches, subscription)
		}
	}

	return &helix.EventSubSubscriptionsResponse{
		ResponseCommon: helix.ResponseCommon{
			StatusCode: http.StatusOK,
		},
		Data: helix.ManyEventSubSubscriptions{
			Total:                 len(matches),
			EventSubSubscriptions: matches,
		},
	}, nil
}

func (m *mockTwitchClient) CreateEventSubSubscription(payload *helix.EventSubSubscription) (*helix.EventSubSubscriptionsResponse, error) {
	if payload.ID != "" {
		return nil, fmt.Errorf("subscription ID should not be specified in CreateEventSubSubscription payload")
	}
	maxId := 10000000
	for _, existing := range m.subscriptions {
		if id, err := strconv.Atoi(existing.ID); err == nil && id > maxId {
			maxId = id
		}
	}
	payload.ID = fmt.Sprintf("%d", maxId+1)

	m.subscriptions = append(m.subscriptions, *payload)
	return &helix.EventSubSubscriptionsResponse{
		ResponseCommon: helix.ResponseCommon{
			StatusCode: http.StatusAccepted,
		},
	}, nil
}

func (m *mockTwitchClient) RemoveEventSubSubscription(id string) (*helix.RemoveEventSubSubscriptionParamsResponse, error) {
	foundAtIndex := -1
	for i := range m.subscriptions {
		if m.subscriptions[i].ID == id {
			foundAtIndex = i
			break
		}
	}

	statusCode := http.StatusNotFound
	if foundAtIndex >= 0 {
		m.subscriptions = append(m.subscriptions[:foundAtIndex], m.subscriptions[foundAtIndex+1:]...)
		statusCode = http.StatusNoContent
	}
	return &helix.RemoveEventSubSubscriptionParamsResponse{
		ResponseCommon: helix.ResponseCommon{
			StatusCode: statusCode,
		},
	}, nil
}

func matchesUserId(subscription *helix.EventSubSubscription, userId string) bool {
	return subscription.Condition.BroadcasterUserID == userId ||
		subscription.Condition.FromBroadcasterUserID == userId ||
		subscription.Condition.ModeratorUserID == userId ||
		subscription.Condition.ToBroadcasterUserID == userId ||
		subscription.Condition.UserID == userId
}
