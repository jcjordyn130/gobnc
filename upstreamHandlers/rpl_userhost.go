package upstreamHandlers

import (
	"github.com/ergochat/irc-go/ircmsg"
)

func Handle302(b Router, msg ircmsg.Message) error {
	// Forward message to client
	b.BroadcastToClients(msg)
	return nil
}
