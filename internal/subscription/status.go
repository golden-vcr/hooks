package subscription

import (
	"fmt"
	"net/http"

	"github.com/golden-vcr/hooks"
	"github.com/nicklaw5/helix/v2"
)

// getOwnedSubscriptions queries the Twitch API to find all relevant EventSub
// subscriptions that are registered with the given user ID and webhook callback URL
func getOwnedSubscriptions(c TwitchClient, channelUserId, callbackUrl string) ([]helix.EventSubSubscription, error) {
	subscriptions := make([]helix.EventSubSubscription, 0)
	params := &helix.EventSubSubscriptionsParams{
		UserID: channelUserId,
	}
	for {
		// Query the Twitch API for a list of our EventSub subscriptions
		r, err := c.GetEventSubSubscriptions(params)
		if err != nil {
			return nil, err
		}
		if r.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("got response %d from get subscriptions request: %s", r.StatusCode, r.ErrorMessage)
		}

		for i := range r.Data.EventSubSubscriptions {
			// Ignore any subscriptions that don't hit our webhook API
			subscription := r.Data.EventSubSubscriptions[i]
			if subscription.Transport.Method != "webhook" {
				continue
			}
			if subscription.Transport.Callback != callbackUrl {
				continue
			}
			subscriptions = append(subscriptions, subscription)
		}

		// Continue making requests until we've seen all subscriptions
		if r.Data.Pagination.Cursor == "" {
			break
		}
		params.After = r.Data.Pagination.Cursor
	}
	return subscriptions, nil
}

// reconcileSubscriptionStatus examines the set of extant EventSub subscriptions as
// returned by the Twitch API, and it compares those subscriptions against the set of
// required subscriptions in order to determine the status of each required subscription
func reconcileSubscriptionStatus(subscriptions []helix.EventSubSubscription, params hooks.RequiredSubscriptionConditionParams, requiredSubscriptions hooks.RequiredSubscriptions) (*Status, error) {
	// Prepare a list that will summarize the details of all subscriptions germane to
	// our hooks service
	subscriptionStates := make([]State, 0, len(requiredSubscriptions))
	unexamined := subscriptions

	// First iterate through all required subscriptions to see if we have an extant
	// EventSub subscription matching the desired type, version, and condition params
	for _, required := range requiredSubscriptions {
		// Substitute template params to get the concrete condition we need our
		// subscription to have
		requiredCondition, err := params.Format(&required.TemplatedCondition)
		if err != nil {
			return nil, fmt.Errorf("failed to format templated condition with params %+v: %v", params, err)
		}

		// Check through all existing subscriptions (as returned by the Twitch API) to
		// find one that matches this requirement
		foundAtIndex := -1
		for i := range unexamined {
			if unexamined[i].Type != required.Type {
				continue
			}
			if unexamined[i].Version != required.Version {
				continue
			}
			if unexamined[i].Condition != *requiredCondition {
				continue
			}
			foundAtIndex = i
			break
		}

		// If we found a valid subscription, add it to our result list with its current
		// status as reported by the Twitch API; otherwise include it as missing (i.e.
		// we require such a subscription, but no such subscription exists)
		status := "missing"
		subscriptionId := ""
		if foundAtIndex >= 0 {
			status = unexamined[foundAtIndex].Status
			subscriptionId = unexamined[foundAtIndex].ID
			unexamined = append(unexamined[:foundAtIndex], unexamined[foundAtIndex+1:]...)
		}
		subscriptionStates = append(subscriptionStates, State{
			Required:       true,
			Type:           required.Type,
			Version:        required.Version,
			Condition:      formatCondition(requiredCondition),
			Status:         status,
			subscriptionId: subscriptionId,
		})
	}

	// If any other subscriptions exist that we don't actually require (and they're
	// registered to the callback URL associated with this service), list them as
	// ancillary
	for _, subscription := range unexamined {
		subscriptionStates = append(subscriptionStates, State{
			Required:       false,
			Type:           subscription.Type,
			Version:        subscription.Version,
			Condition:      formatCondition(&subscription.Condition),
			Status:         subscription.Status,
			subscriptionId: subscription.ID,
		})
	}

	// Determine if our EventSub subscription status is A-OK: for full backend
	// functionality to work as intended, all required subscriptions must exist and have
	// a status of "enabled"
	ok := true
	for _, state := range subscriptionStates {
		if state.Required && state.Status != "enabled" {
			ok = false
			break
		}
	}

	// Return a struct that represents the overall status of our hooks service's
	// EventSub subscriptions
	return &Status{
		Ok:            ok,
		Subscriptions: subscriptionStates,
	}, nil
}
