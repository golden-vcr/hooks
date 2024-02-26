package main

import (
	"encoding/json"
	"flag"
	"strings"

	"github.com/nicklaw5/helix/v2"
)

func initHypeCommand(cmd *flag.FlagSet) {
}

func runHypeCommand(channelName, channelUserId string) (string, json.RawMessage) {
	ev, err := json.Marshal(helix.EventSubHypeTrainBeginEvent{
		BroadcasterUserID:    channelUserId,
		BroadcasterUserLogin: strings.ToLower(channelName),
		BroadcasterUserName:  channelName,
		Total:                137,
		Progress:             137,
		Goal:                 500,
	})
	if err != nil {
		panic(err)
	}
	return helix.EventSubTypeHypeTrainBegin, ev
}
