package upstreamHandlers

import (
	"github.com/ergochat/irc-go/ircmsg"
)

// RPL_306 Client Away
func Handle306(b Router, msg ircmsg.Message) (bool, error) {
	// Forward message to client
	b.BroadcastToClients(msg)
	return true, nil
}

// RPL_305 Client Away Unset
func Handle305(b Router, msg ircmsg.Message) (bool, error) {
	// Forward message to client
	b.BroadcastToClients(msg)
	return true, nil
}
