package main

import (
	"encoding/json"
	"flag"
	"strings"

	"github.com/nicklaw5/helix/v2"
)

func initOfflineCommand(cmd *flag.FlagSet) {
}

func runOfflineCommand(channelName, channelUserId string) (string, json.RawMessage) {
	ev, err := json.Marshal(helix.EventSubStreamOfflineEvent{
		BroadcasterUserID:    channelUserId,
		BroadcasterUserLogin: strings.ToLower(channelName),
		BroadcasterUserName:  channelName,
	})
	if err != nil {
		panic(err)
	}
	return helix.EventSubTypeStreamOffline, ev
}
