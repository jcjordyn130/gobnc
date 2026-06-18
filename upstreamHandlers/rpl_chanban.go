package upstreamHandlers

import (
	"log"

	"github.com/ergochat/irc-go/ircmsg"
)

func Handle474(b Router, msg ircmsg.Message) error {
	// Sanity check
	if len(msg.Params) < 2 {
		log.Printf("[upstream %s] Ignoring RPL 474 due to invalid paramaters", b.GetUpstreamConn().Server)
		return nil
	}

	channel := msg.Params[1]
	log.Printf("[upstream %s] Cannot join channel %s due to ban", b.GetUpstreamConn().Server, channel)

	// Forward message to client
	b.BroadcastToClients(msg)

	return nil
}
