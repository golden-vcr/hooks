package callback

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/golden-vcr/server-common/entry"
	"github.com/gorilla/mux"
	"github.com/nicklaw5/helix/v2"
)

type VerifyNotificationFunc func(header http.Header, message string) bool
type HandleEventFunc func(ctx context.Context, subscription *helix.EventSubSubscription, data json.RawMessage) error

type Server struct {
	verifyNotification VerifyNotificationFunc
	handleEvent        HandleEventFunc
}

func NewServer(twitchWebhookSecret string) *Server {
	return &Server{
		verifyNotification: func(header http.Header, message string) bool {
			hmacMessage := []byte(fmt.Sprintf("%s%s%s", header.Get("Twitch-Eventsub-Message-Id"), header.Get("Twitch-Eventsub-Message-Timestamp"), message))
			mac := hmac.New(sha256.New, []byte(twitchWebhookSecret))
			mac.Write(hmacMessage)
			hmacsha256 := fmt.Sprintf("sha256=%s", hex.EncodeToString(mac.Sum(nil)))
			fmt.Printf("|| Verifying EventSub notification...\n")
			fmt.Printf("|| -        Twitch-Eventsub-Message-Id: %s\n", header.Get("Twitch-Eventsub-Message-Id"))
			fmt.Printf("|| - Twitch-Eventsub-Message-Timestamp: %s\n", header.Get("Twitch-Eventsub-Message-Timestamp"))
			fmt.Printf("|| -                           message: '%s'\n", string(message))
			fmt.Printf("|| -                            secret: '%s'\n", twitchWebhookSecret)
			fmt.Printf("|| -                          computed: %s\n", hmacsha256)
			fmt.Printf("|| - Twitch-Eventsub-Message-Signature: %s\n", header.Get("Twitch-Eventsub-Message-Signature"))
			return helix.VerifyEventSubNotification(twitchWebhookSecret, header, message)
		},
		handleEvent: func(ctx context.Context, subscription *helix.EventSubSubscription, data json.RawMessage) error {
			// TODO: Something
			return nil
		},
	}
}

func (s *Server) RegisterRoutes(r *mux.Router) {
	r.Path("/callback").Methods("POST").HandlerFunc(s.handlePostCallback)
}

func (s *Server) handlePostCallback(res http.ResponseWriter, req *http.Request) {
	logger := entry.Log(req)

	// Pre-emptively read the request body so we can verify its signature
	body, err := io.ReadAll(req.Body)
	if err != nil {
		logger.Error("Failed to read request body", "error", err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}
	defer req.Body.Close()

	// Verify that this event comes from Twitch: abort if phony
	if !s.verifyNotification(req.Header, string(body)) {
		logger.Error("Failed to verify signature")
		http.Error(res, "Signature verification failed", http.StatusBadRequest)
		return
	}

	// Decode the payload from JSON so we can examine the details of the event
	var payload struct {
		Subscription helix.EventSubSubscription `json:"subscription"`
		Challenge    string                     `json:"challenge"`
		Event        json.RawMessage            `json:"event"`
	}
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&payload); err != nil {
		logger.Error("Failed to decode request body")
		http.Error(res, err.Error(), http.StatusBadRequest)
		return
	}

	// If the challenge value is set, Twitch is sending us an initial request to
	// confirm registration of this event callback: responding with the same value will
	// enable the event subscription. This occurs after the parseEvent check so that we
	// won't allow subscriptions to be created until we fully support the relevant
	// event type.
	if payload.Challenge != "" {
		logger.Error("Responding to challenge", "challenge", payload.Challenge)
		res.Write([]byte(payload.Challenge))
		return
	}

	// Attempt to handle the event, using our HandleEventFunc: this should be relatively
	// lightweight, since we're doing it synchronously in the callback handler and
	// waiting to respond to Twitch until finished
	logger = logger.With(
		"subscriptionId", payload.Subscription.ID,
		"subscriptionType", payload.Subscription.Type,
		"subscriptionVersion", payload.Subscription.Version,
		"event", string(payload.Event),
	)
	if err := s.handleEvent(req.Context(), &payload.Subscription, payload.Event); err != nil {
		logger.Error("Failed to handle event", "error", err)
		http.Error(res, err.Error(), http.StatusInternalServerError)
		return
	}

	// If successful, write a 200 response and we're done
	logger.Info("Handled event")
	res.WriteHeader(http.StatusOK)
}
