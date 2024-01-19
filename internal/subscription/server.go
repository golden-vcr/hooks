package subscription

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/golden-vcr/auth"
	"github.com/golden-vcr/hooks"
	"github.com/golden-vcr/server-common/entry"
	"github.com/golden-vcr/server-common/twitch"
	"github.com/gorilla/mux"
	"github.com/nicklaw5/helix/v2"
)

type NewTwitchClientFunc func(ctx context.Context) (TwitchClient, error)

type Server struct {
	callbackUrl           string
	conditionParams       hooks.RequiredSubscriptionConditionParams
	requiredSubscriptions hooks.RequiredSubscriptions

	newTwitchClient     NewTwitchClientFunc
	twitchWebhookSecret string
}

func NewServer(origin, twitchChannelUserId, twitchClientId, twitchClientSecret, twitchWebhookSecret string) *Server {
	return &Server{
		callbackUrl: origin + "/callback",
		conditionParams: hooks.RequiredSubscriptionConditionParams{
			ChannelUserId: twitchChannelUserId,
		},
		requiredSubscriptions: hooks.Subscriptions,
		newTwitchClient: func(ctx context.Context) (TwitchClient, error) {
			return twitch.NewClientWithAppToken(ctx, twitchClientId, twitchClientSecret)
		},
		twitchWebhookSecret: twitchWebhookSecret,
	}
}

func (s *Server) RegisterRoutes(c auth.Client, r *mux.Router) {
	subscriptions := r.Path("/subscriptions").Subrouter()
	subscriptions.Use(func(next http.Handler) http.Handler {
		return auth.RequireAccess(c, auth.RoleBroadcaster, next)
	})
	subscriptions.Methods("GET").HandlerFunc(s.handleGetSubscriptions)
	subscriptions.Methods("PATCH").HandlerFunc(s.handlePatchSubscriptions)
	subscriptions.Methods("DELETE").HandlerFunc(s.handleDeleteSubscriptions)
}

// handleGetSubscriptions (GET /subscriptions) queries the Twitch API to return the
// current status of all subscriptions required by and/or registered to this service:
// only subscriptions associated with this service's callback URL are included
func (s *Server) handleGetSubscriptions(res http.ResponseWriter, req *http.Request) {
	logger := entry.Log(req)

	c, err := s.newTwitchClient(req.Context())
	if err != nil {
		logger.Error("Failed to initialize Twitch API client", "error", err)
		http.Error(res, fmt.Sprintf("failed to initialize Twitch API client: %v", err), http.StatusInternalServerError)
		return
	}

	status, err := s.fetchSubscriptionStatus(c)
	if err != nil {
		logger.Error("Failed to resolve EventSub subscription status", "error", err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := json.NewEncoder(res).Encode(status); err != nil {
		http.Error(res, err.Error(), http.StatusInternalServerError)
	}
}

// handlePatchSubscriptions (PATCH /subscriptions) attempts to register all required
// EventSub subscriptions that are not currently registered
func (s *Server) handlePatchSubscriptions(res http.ResponseWriter, req *http.Request) {
	logger := entry.Log(req)

	c, err := s.newTwitchClient(req.Context())
	if err != nil {
		logger.Error("Failed to initialize Twitch API client", "error", err)
		http.Error(res, fmt.Sprintf("failed to initialize Twitch API client: %v", err), http.StatusInternalServerError)
		return
	}

	status, err := s.fetchSubscriptionStatus(c)
	if err != nil {
		logger.Error("Failed to resolve EventSub subscription status", "error", err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	for _, subscription := range status.Subscriptions {
		if subscription.Required && subscription.Status == "missing" {
			if err := s.createSubscription(c, subscription.Type, subscription.Version, subscription.Condition); err != nil {
				logger.Error("Failed to create EventSub subscription",
					"error", err,
					"subscriptionType", subscription.Type,
					"subscriptionVersion", subscription.Version,
					"subscriptionCondition", subscription.Condition,
				)
				http.Error(res, fmt.Sprintf("Failed to create EventSub subscription: %v", err), http.StatusInternalServerError)
				return
			}
			logger.Info("Created new EventSub subscription",
				"subscriptionType", subscription.Type,
				"subscriptionVersion", subscription.Version,
				"subscriptionCondition", subscription.Condition,
			)
		}
	}
	res.WriteHeader(http.StatusNoContent)
}

// handleDeleteSubscriptions (DELETE /subscriptions) deletes ALL EventSub subscriptions
// that have been registered to the callback URL associated with this service
func (s *Server) handleDeleteSubscriptions(res http.ResponseWriter, req *http.Request) {
	logger := entry.Log(req)

	c, err := s.newTwitchClient(req.Context())
	if err != nil {
		logger.Error("Failed to initialize Twitch API client", "error", err)
		http.Error(res, fmt.Sprintf("failed to initialize Twitch API client: %v", err), http.StatusInternalServerError)
		return
	}

	status, err := s.fetchSubscriptionStatus(c)
	if err != nil {
		logger.Error("Failed to resolve EventSub subscription status", "error", err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	for _, subscription := range status.Subscriptions {
		if subscription.subscriptionId != "" {
			if err := s.deleteSubscription(c, subscription.subscriptionId); err != nil {
				logger.Error("Failed to delete EventSub subscription",
					"error", err,
					"subscriptionId", subscription.subscriptionId,
					"subscriptionType", subscription.Type,
					"subscriptionVersion", subscription.Version,
					"subscriptionCondition", subscription.Condition,
				)
				http.Error(res, fmt.Sprintf("Failed to delete EventSub subscription: %v", err), http.StatusInternalServerError)
				return
			}
			logger.Info("Deleted EventSub subscription",
				"subscriptionId", subscription.subscriptionId,
				"subscriptionType", subscription.Type,
				"subscriptionVersion", subscription.Version,
				"subscriptionCondition", subscription.Condition,
			)
		}
	}
	res.WriteHeader(http.StatusNoContent)
}

// fetchSubscriptionStatus gets current EventSub subscription state from the Twitch API,
// then reconciles it against the set of required subscriptions in order to resolve a
// subscription.Status struct describing the overall state of all EventSub subscriptions
// related to this service
func (s *Server) fetchSubscriptionStatus(c TwitchClient) (*Status, error) {
	subscriptions, err := getOwnedSubscriptions(c, s.conditionParams.ChannelUserId, s.callbackUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to get EventSub subscriptions: %w", err)
	}

	status, err := reconcileSubscriptionStatus(subscriptions, s.conditionParams, s.requiredSubscriptions)
	if err != nil {
		return nil, fmt.Errorf("failed to reconcile EventSub subscription status: %w", err)
	}

	return status, nil
}

// createSubscription uses the Twitch API to register a new EventSub subscription with
// the given parameters, configured appropriately to register a webhook callback with
// this service
func (s *Server) createSubscription(c TwitchClient, subscriptionType string, version string, condition map[string]string) error {
	r, err := c.CreateEventSubSubscription(&helix.EventSubSubscription{
		Type:      subscriptionType,
		Version:   version,
		Condition: parseCondition(condition),
		Transport: helix.EventSubTransport{
			Method:   "webhook",
			Callback: s.callbackUrl,
			Secret:   s.twitchWebhookSecret,
		},
	})
	if err != nil {
		return err
	}
	if r.StatusCode != http.StatusAccepted {
		return fmt.Errorf("got response %d from CreateEventSubSubscription request: %s", r.StatusCode, r.ErrorMessage)
	}
	return nil
}

// deleteSubscription uses the Twitch API to remove a single EventSub subscription,
// given its ID
func (s *Server) deleteSubscription(c TwitchClient, subscriptionId string) error {
	r, err := c.RemoveEventSubSubscription(subscriptionId)
	if err != nil {
		return err
	}
	if r.StatusCode != http.StatusNoContent {
		return fmt.Errorf("got response %d from RemoveEventSubSubscription request: %s", r.StatusCode, r.ErrorMessage)
	}
	return nil
}
