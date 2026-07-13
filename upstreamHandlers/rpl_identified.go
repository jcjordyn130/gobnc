package upstreamHandlers

import (
	"github.com/ergochat/irc-go/ircmsg"
)

func Handle900(b Router, msg ircmsg.Message) (bool, error) {
	// Forward message to client
	b.BroadcastToClients(msg)
	return true, nil
}
