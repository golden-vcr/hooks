package main

import (
	"encoding/json"
	"flag"
	"strings"

	"github.com/nicklaw5/helix/v2"
)

var cheerUsername string
var cheerUserId string
var cheerNumBits int
var cheerMessage string

func initCheerCommand(cmd *flag.FlagSet) {
	cmd.StringVar(&cheerUsername, "username", "BigJoeBob", "Twitch Display Name indicating who has followed the channel")
	cmd.StringVar(&cheerUserId, "user-id", "1337", "Twitch User ID of the user that followed the channel")
	cmd.IntVar(&cheerNumBits, "num-bits", 200, "Number of bits cheered")
	cmd.StringVar(&cheerMessage, "message", "", "Text of cheer message")
}

func runCheerCommand(channelName, channelUserId string) (string, json.RawMessage) {
	ev, err := json.Marshal(helix.EventSubChannelCheerEvent{
		UserID:               cheerUserId,
		UserLogin:            strings.ToLower(cheerUsername),
		UserName:             cheerUsername,
		BroadcasterUserID:    channelUserId,
		BroadcasterUserLogin: strings.ToLower(channelName),
		BroadcasterUserName:  channelName,
		Message:              cheerMessage,
		Bits:                 cheerNumBits,
	})
	if err != nil {
		panic(err)
	}
	return helix.EventSubTypeChannelCheer, ev
}
