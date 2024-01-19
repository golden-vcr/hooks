// Package userauth contains code used to redirect the user to an Twitch-hosted OAuth
// challenge where they can grant our application access to their Twitch channel with
// all the requisite scopes required by our backend.
//
// All calls to the EventSub API are authorized using an application access token: this
// token (which we obtain using our app's client ID and client secret) is what
// identifies our app when registering an EventSub webhook subscription.
//
// Separately, though, certain EventSub subscription types require that our app has been
// granted access to a target channel with a specific set of OAuth scopes. For example,
// if we attempt to register a 'channel.follow' callback on a channel with a Twitch user
// ID of "12345", Twitch will check to see whether our app has been connected to that
// Twitch account with the 'moderator:read:followers' scope. If so, the request will
// succeed and the subscription will be registered; if not, the request will fail with a
// 403 error.
//
// In order for the user (i.e. the owner of the target channel, i.e. the broadcaster) to
// grant access to our application, we need to direct that user to an OAuth challenge
// page for our app, by sending them to id.twitch.tv and initiating an Authorization
// code grant flow as described here:
//
// - https://dev.twitch.tv/docs/authentication/getting-tokens-oauth/#authorization-code-grant-flow
//
// As an end result of this process, we'll end up with a Twitch User Access Token that
// includes all the requisite scopes required to register our EventSub subscriptions.
// We don't actually use that User Access Token anywhere; all we care about is the side
// effect on the Twitch backend of establishing that our app has the required level of
// access to our target channel.
package userauth
