# hooks

The **hooks** application is an HTTP server that handles EventSub webhook callbacks from
Twitch, allowing our backend to respond to events that occur on Twitch, such as the
channel status changing, a viewer following or subscribing, etc.

- **OpenAPI specification:** https://golden-vcr.github.io/hooks/

When the **hooks** server receives a valid callback from Twitch, it generates an
internal event and pushes it onto a queue. Further downstream in the backend, other
applications consume those events in order to record and respond to state changes
independently.

## Prerequisites

Install [Go 1.21](https://go.dev/doc/install). If successful, you should be able to run:

```
> go version
go version go1.21.0 windows/amd64
```

## Initial setup

Create a file in the root of this repo called `.env` that contains the environment
variables required in [`main.go`](./cmd/server/main.go). If you have the
[`terraform`](https://github.com/golden-vcr/terraform) repo cloned alongside this one,
simply open a shell there and run:

- `terraform output -raw twitch_api_env > ../hooks/.env`

## Registering EventSub subscriptions

Once the application is deployed, an accompanying frontend allows an admin (i.e. a user
logged into the webapp with broadcaster access) to view the status of required EventSub
subscriptions, and to delete or create those subscriptions as needed. See:

- https://goldenvcr.com/admin/hooks
