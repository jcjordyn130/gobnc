package upstreamHandlers

import (
	"github.com/ergochat/irc-go/ircmsg"
)

func HandleJOIN(b Router, msg ircmsg.Message) error {
	// TODO: update nick list

	// Forward message to client
	b.BroadcastToClients(msg)

	return nil
}
