package upstreamHandlers

import (
	"github.com/ergochat/irc-go/ircmsg"
)

func HandlePART(b Router, msg ircmsg.Message) error {
	// Clients MUST receieve their own PART
	// Forward message to client
	b.BroadcastToClients(msg)

	return nil
}
