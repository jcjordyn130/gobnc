package upstreamHandlers

import (
	"github.com/ergochat/irc-go/ircmsg"
)

func HandleNOTICE(b Router, msg ircmsg.Message) (bool, error) {
	// Write message to database for backlog
	b.LogToDB(msg)

	// Broadcast to clients
	b.BroadcastToClients(msg)

	return true, nil
}
