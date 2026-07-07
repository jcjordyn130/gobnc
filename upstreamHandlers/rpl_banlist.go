package upstreamHandlers

import (
	"github.com/ergochat/irc-go/ircmsg"
)

func Handle367(b Router, msg ircmsg.Message) error {
	// Forward message to client
	b.BroadcastToClients(msg)

	return nil
}

func Handle368(b Router, msg ircmsg.Message) error {
	// Forward message to client
	b.BroadcastToClients(msg)

	return nil
}
