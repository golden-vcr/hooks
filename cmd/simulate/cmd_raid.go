package main

import (
	"encoding/json"
	"flag"
	"strings"

	"github.com/nicklaw5/helix/v2"
)

var raidUsername string
var raidUserId string
var raidNumViewers int

func initRaidCommand(cmd *flag.FlagSet) {
	cmd.StringVar(&raidUsername, "username", "BigJoeBob", "Twitch Display Name indicating who has raided the channel")
	cmd.StringVar(&raidUserId, "user-id", "1337", "Twitch User ID of the user that raided the channel")
	cmd.IntVar(&raidNumViewers, "num-viewers", 99, "Number of viewers in the raid")
}

func runRaidCommand(channelName, channelUserId string) (string, json.RawMessage) {
	ev, err := json.Marshal(helix.EventSubChannelRaidEvent{
		FromBroadcasterUserID:    raidUserId,
		FromBroadcasterUserLogin: strings.ToLower(raidUsername),
		FromBroadcasterUserName:  raidUsername,
		ToBroadcasterUserID:      channelUserId,
		ToBroadcasterUserLogin:   strings.ToLower(channelName),
		ToBroadcasterUserName:    channelName,
		Viewers:                  raidNumViewers,
	})
	if err != nil {
		panic(err)
	}
	return helix.EventSubTypeChannelRaid, ev
}
