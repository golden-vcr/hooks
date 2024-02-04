# hooks

The **hooks** application is an HTTP server that handles EventSub webhook callbacks from
Twitch, allowing our backend to respond to events that occur on Twitch, such as the
channel status changing, a viewer following or subscribing, etc.

- **OpenAPI specification:** https://golden-vcr.github.io/hooks/

When the **hooks** server receives a valid callback from Twitch, it generates an
internal event and pushes it onto a queue. Further downstream in the backend, other
applications consume those events in order to record and respond to state changes
independently.

## Development Guide

On a Linux or WSL system:

1. Install [Go 1.21](https://go.dev/doc/install)
2. Clone the [**terraform**](https://github.com/golden-vcr/terraform) repo alongside
   this one, and from the root of that repo:
    - Ensure that the module is initialized (via `terraform init`)
    - Ensure that valid terraform state is present
    - Run `terraform output -raw env_hooks_local > ../hooks/.env` to populate an `.env`
      file.
    - Run [`./local-rmq.sh up`](https://github.com/golden-vcr/terraform/blob/main/local-rmq.sh)
      to ensure that a RabbitMQ server is running locally (requires
      [Docker](https://docs.docker.com/engine/install/)).
3. Ensure that the [**auth**](https://github.com/golden-vcr/auth?tab=readme-ov-file#development-guide)
   server is running locally.
4. From the root of this repository:
    - Run [`go run ./cmd/server`](./cmd/server/main.go) to start up the server.

Once done, the hooks server will be running at http://localhost:5003.

## Simulating events locally

Note that the locally-running hooks server will _not_ receive webhook callbacks from
Twitch - we only register actual Twitch EventSub callbacks using the canonical public
URL associated with our platform, i.e. https://goldenvcr.com.

Once you're running the hooks server locally as described above, you can simulate
Twitch-instigated events instead: simply run [`go run ./cmd/simulate`](./cmd/simulate/main.go)
to see a list of subcommands corresponding to different event types, and invoke each
subcommand with `-h` to see a list of configurable parameters.

For example, to simulate a raid from `tsjonte` with 69 viewers, run:

- `go run ./cmd/simulate raid -username tsjonte -user-id 37071883 -num-viewers 69`

## Registering EventSub subscriptions

Once the application is deployed to a live environment, an accompanying frontend allows
an admin (i.e. a user logged into the webapp with broadcaster access) to view the status
of required EventSub subscriptions, and to delete or create those subscriptions as
needed. To register live webhook callbacks from Twitch to goldenvcr.com, visit:

- https://goldenvcr.com/admin/hooks
