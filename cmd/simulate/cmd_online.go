package main

import (
	"encoding/json"
	"flag"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nicklaw5/helix/v2"
)

func initOnlineCommnand(cmd *flag.FlagSet) {
}

func runOnlineCommand(channelName, channelUserId string) (string, json.RawMessage) {
	ev, err := json.Marshal(helix.EventSubStreamOnlineEvent{
		ID:                   uuid.NewString(),
		BroadcasterUserID:    channelUserId,
		BroadcasterUserLogin: strings.ToLower(channelName),
		BroadcasterUserName:  channelName,
		Type:                 "live",
		StartedAt:            helix.Time{Time: time.Now()},
	})
	if err != nil {
		panic(err)
	}
	return helix.EventSubTypeStreamOnline, ev
}
