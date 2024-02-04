package main

import (
	"encoding/json"
	"flag"
	"strings"
	"time"

	"github.com/nicklaw5/helix/v2"
)

var followUsername string
var followUserId string

func initFollowCommand(cmd *flag.FlagSet) {
	cmd.StringVar(&followUsername, "username", "BigJoeBob", "Twitch Display Name indicating who has followed the channel")
	cmd.StringVar(&followUserId, "user-id", "1337", "Twitch User ID of the user that followed the channel")
}

func runFollowCommand(channelName, channelUserId string) (string, json.RawMessage) {
	ev, err := json.Marshal(helix.EventSubChannelFollowEvent{
		UserID:               followUserId,
		UserLogin:            strings.ToLower(followUsername),
		UserName:             followUsername,
		BroadcasterUserID:    channelUserId,
		BroadcasterUserLogin: strings.ToLower(channelName),
		BroadcasterUserName:  channelName,
		FollowedAt:           helix.Time{Time: time.Now()},
	})
	if err != nil {
		panic(err)
	}
	return helix.EventSubTypeChannelFollow, ev
}
