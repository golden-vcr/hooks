package main

import (
	"fmt"
	"os"

	"github.com/codingconcepts/env"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/golden-vcr/auth"
	"github.com/golden-vcr/hooks/internal/callback"
	"github.com/golden-vcr/hooks/internal/subscription"
	"github.com/golden-vcr/hooks/internal/userauth"
	"github.com/golden-vcr/server-common/entry"
	"github.com/golden-vcr/server-common/rmq"
	"github.com/golden-vcr/server-common/twitch"
)

type Config struct {
	BindAddr   string `env:"BIND_ADDR"`
	ListenPort uint16 `env:"LISTEN_PORT" default:"5004"`
	Origin     string `env:"ORIGIN" default:"https://goldenvcr.com/api/hooks"`

	TwitchChannelName   string `env:"TWITCH_CHANNEL_NAME" required:"true"`
	TwitchClientId      string `env:"TWITCH_CLIENT_ID" required:"true"`
	TwitchClientSecret  string `env:"TWITCH_CLIENT_SECRET" required:"true"`
	TwitchWebhookSecret string `env:"TWITCH_WEBHOOK_SECRET" required:"true"`

	RmqHost     string `env:"RMQ_HOST" required:"true"`
	RmqPort     int    `env:"RMQ_PORT" required:"true"`
	RmqVhost    string `env:"RMQ_VHOST" required:"true"`
	RmqUser     string `env:"RMQ_USER" required:"true"`
	RmqPassword string `env:"RMQ_PASSWORD" required:"true"`

	AuthURL string `env:"AUTH_URL" default:"http://localhost:5002"`
}

func main() {
	app, ctx := entry.NewApplication("hooks")
	defer app.Stop()

	// Parse config from environment variables
	err := godotenv.Load()
	if err != nil && !os.IsNotExist(err) {
		app.Fail("Failed to load .env file", err)
	}
	config := Config{}
	if err := env.Set(&config); err != nil {
		app.Fail("Failed to load config", err)
	}

	// Initialize an AMQP client
	amqpConn, err := amqp.Dial(rmq.FormatConnectionString(config.RmqHost, config.RmqPort, config.RmqVhost, config.RmqUser, config.RmqPassword))
	if err != nil {
		app.Fail("Failed to connect to AMQP server", err)
	}
	producer, err := rmq.NewProducer(amqpConn, "twitch-events")
	if err != nil {
		app.Fail("Failed to initialize AMQP producer", err)
	}

	// Initialize an auth client so we can require broadcaster-level access in order to
	// call the admin-only subscription management endpoints
	authClient, err := auth.NewClient(ctx, config.AuthURL)
	if err != nil {
		app.Fail("Failed to initialize auth client", err)
	}

	// Initialize a Twitch API client with an app access token, then use it to resolve
	// the Twitch User ID of our desired channel
	twitchClient, err := twitch.NewClientWithAppToken(ctx, config.TwitchClientId, config.TwitchClientSecret)
	if err != nil {
		app.Fail("Failed to initialize Twitch API client", err)
	}
	channelUserId, err := twitch.ResolveChannelUserId(twitchClient, config.TwitchChannelName)
	if err != nil {
		app.Fail(fmt.Sprintf("Failed to resolve Twitch user ID for channel '%s'", config.TwitchChannelName), err)
	}
	app.Log().Info(
		"Initialized broadcaster channel details",
		"channelName", config.TwitchChannelName,
		"channelUserId", channelUserId,
	)

	// Start setting up our HTTP handlers, using gorilla/mux for routing
	r := mux.NewRouter()

	// Twitch will call POST /callback (once we've registered EventSub subscriptions
	// configuring it to do so) in response to events that occur on Twitch
	callbackServer := callback.NewServer(config.TwitchWebhookSecret, producer)
	callbackServer.RegisterRoutes(r)

	// A client authenticated as the broadcaster can call GET /subscriptions to view the
	// status of required EventSub subscriptions, PATCH to create ones that are missing,
	// and DELETE to remove them all
	subscriptionServer := subscription.NewServer(
		config.Origin,
		channelUserId,
		config.TwitchClientId,
		config.TwitchClientSecret,
		config.TwitchWebhookSecret,
	)
	subscriptionServer.RegisterRoutes(authClient, r)

	// Registering EventSub subscriptions requires that our application be connected to
	// the target Twitch channel: the broadcaster can GET /userauth/start to initiate an
	// OAuth code grant flow that will accomplish that, and redirect_uri for that flow
	// will send an authorization code back to GET /userauth/finish
	userauthServer := userauth.NewServer(config.Origin, config.TwitchClientId)
	userauthServer.RegisterRoutes(r)

	// Handle incoming HTTP connections until our top-level context is canceled, at
	// which point shut down cleanly
	entry.RunServer(ctx, app.Log(), r, config.BindAddr, config.ListenPort)
}
