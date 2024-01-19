package subscription

import "github.com/nicklaw5/helix/v2"

// TwitchClient represents the subset of Twitch API client functionality used to view
// and manage the state of EventSub subscriptions
type TwitchClient interface {
	GetEventSubSubscriptions(params *helix.EventSubSubscriptionsParams) (*helix.EventSubSubscriptionsResponse, error)
	CreateEventSubSubscription(payload *helix.EventSubSubscription) (*helix.EventSubSubscriptionsResponse, error)
	RemoveEventSubSubscription(id string) (*helix.RemoveEventSubSubscriptionParamsResponse, error)
}
