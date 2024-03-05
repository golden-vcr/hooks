package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/codingconcepts/env"
	"github.com/golden-vcr/hooks"
	"github.com/golden-vcr/server-common/twitch"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/nicklaw5/helix/v2"
)

const (
	TwitchHeaderMessageId        = "twitch-eventsub-message-id"
	TwitchHeaderMessageTimestamp = "twitch-eventsub-message-timestamp"
	TwitchHeaderMessageSignature = "twitch-eventsub-message-signature"
)

type Config struct {
	TwitchChannelName   string `env:"TWITCH_CHANNEL_NAME" required:"true"`
	TwitchClientId      string `env:"TWITCH_CLIENT_ID" required:"true"`
	TwitchClientSecret  string `env:"TWITCH_CLIENT_SECRET" required:"true"`
	TwitchWebhookSecret string `env:"TWITCH_WEBHOOK_SECRET" required:"true"`
}

type MessagePayload struct {
	Subscription helix.EventSubSubscription `json:"subscription"`
	Challenge    string                     `json:"challenge"`
	Event        json.RawMessage            `json:"event"`
}

type Command struct {
	name     string
	initFunc func(cmd *flag.FlagSet)
	runFunc  func(channelName, channelUserId string) (string, json.RawMessage)
}

var commands = []Command{
	{"online", initOnlineCommnand, runOnlineCommand},
	{"offline", initOfflineCommand, runOfflineCommand},
	{"hype", initHypeCommand, runHypeCommand},
	{"follow", initFollowCommand, runFollowCommand},
	{"raid", initRaidCommand, runRaidCommand},
	{"cheer", initCheerCommand, runCheerCommand},
}

func main() {
	// We only want to simulate events locally for now; events that can be recorded in
	// the production DB and affect the state of the actual, deployed webapp should only
	// come from Twitch itself
	url := "http://localhost:5004/callback"

	// Parse config from environment variables
	err := godotenv.Load()
	if err != nil && !os.IsNotExist(err) {
		log.Fatalf("error loading .env file: %v", err)
	}
	config := Config{}
	if err := env.Set(&config); err != nil {
		log.Fatalf("error loading config: %v", err)
	}

	// Initialize a Twitch API client with an app access token, then use it to resolve
	// the Twitch User ID of our desired channel
	twitchClient, err := twitch.NewClientWithAppToken(context.Background(), config.TwitchClientId, config.TwitchClientSecret)
	if err != nil {
		log.Fatalf("Failed to initialize Twitch API client: %v", err)
	}
	channelUserId, err := twitch.ResolveChannelUserId(twitchClient, config.TwitchChannelName)
	if err != nil {
		log.Fatalf("Failed to resolve Twitch user ID for channel '%s': %v", config.TwitchChannelName, err)
	}

	// Parse the subcommand that we want to run, or print usage if no match
	var command *Command
	commandName := ""
	if len(os.Args) > 1 {
		commandName = os.Args[1]
	}
	for i := range commands {
		if commands[i].name == commandName {
			command = &commands[i]
			break
		}
	}
	if command == nil {
		commandNames := make([]string, 0, len(commands))
		for i := range commands {
			commandNames = append(commandNames, commands[i].name)
		}
		log.Fatalf("Usage: simulate [%s]", strings.Join(commandNames, "|"))
	}

	// Initialize command-line flags for the chosen subcommand
	flagSet := flag.NewFlagSet(command.name, flag.ExitOnError)
	command.initFunc(flagSet)
	if err := flagSet.Parse(os.Args[2:]); err != nil {
		log.Fatalf("Parse error: %v", err)
	}

	// Run the subcommand-specific function to build an event payload
	subscriptionType, event := command.runFunc(config.TwitchChannelName, channelUserId)

	// Build a message payload, finding a required subscription that matches the type
	// indicated by our subcommand
	params := hooks.RequiredSubscriptionConditionParams{ChannelUserId: channelUserId}
	payload := MessagePayload{}
	for _, required := range hooks.Subscriptions {
		if required.Type == subscriptionType {
			payload.Subscription.Type = subscriptionType
			payload.Subscription.Version = required.Version
			cond, err := params.Format(&required.TemplatedCondition)
			if err != nil {
				log.Fatalf("failed to format subscription condition from template: %v", err)
			}
			payload.Subscription.Condition = *cond
			break
		}
	}
	if payload.Subscription.Type == "" {
		log.Fatalf("no subscription of type %s is required by the service", subscriptionType)
	}
	payload.Subscription.ID = uuid.NewString()
	payload.Subscription.Status = helix.EventSubStatusEnabled
	payload.Subscription.Transport.Method = "webhook"
	payload.Subscription.Transport.Callback = url
	payload.Subscription.CreatedAt = helix.Time{Time: time.Now().Add(-5 * time.Minute)}
	payload.Event = event

	// Serialize our entire payload to JSON
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		log.Fatalf("failed to encode message payload: %v", err)
	}
	body := string(bodyBytes)

	// Prepare the HTTP request that will carry that message in its body
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		log.Fatalf("error initializing HTTP request: %v", err)
	}

	// Set Twitch-Eventsub-* headers to identify the message and cryptographically sign
	// it, using the webhook secret, in a way that helix.VerifyEventSubNotification can
	// verify
	req.Header.Set(TwitchHeaderMessageId, uuid.New().String())
	req.Header.Set(TwitchHeaderMessageTimestamp, time.Now().Format(time.RFC3339))
	req.Header.Set(TwitchHeaderMessageSignature, computeSignature(config.TwitchWebhookSecret, req.Header, body))

	// Print the details of the request to stdout
	fmt.Printf("%s %s\n", req.Method, req.URL)
	for k, values := range req.Header {
		for _, v := range values {
			fmt.Printf("> %s: %s\n", k, v)
		}
	}
	pretty, err := json.MarshalIndent(payload, "", "    ")
	if err != nil {
		log.Fatalf("failed to pretty-print JSON payload: %v", err)
	}
	fmt.Printf("\n%s\n\n", pretty)

	// Send the request and verify that we get an OK response
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("error sending HTTP request: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		log.Fatalf("got response %d", res.StatusCode)
	}
	fmt.Printf("< %d\n", res.StatusCode)
}

func computeSignature(secret string, h http.Header, message string) string {
	hmacMessage := []byte(fmt.Sprintf("%s%s%s", h.Get(TwitchHeaderMessageId), h.Get(TwitchHeaderMessageTimestamp), message))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(hmacMessage)
	return fmt.Sprintf("sha256=%s", hex.EncodeToString(mac.Sum(nil)))
}
