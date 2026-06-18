package upstreamHandlers

import (
	"log"

	"github.com/ergochat/irc-go/ircmsg"
)

func Handle329(b Router, msg ircmsg.Message) error {
	// Sanity check
	if len(msg.Params) < 2 {
		log.Printf("[upstream %s] Ignoring RPL 329 due to invalid paramaters", b.GetUpstreamConn().Server)
		return nil
	}

	channel := msg.Params[1]
	createTime := msg.Params[2]

	log.Printf("[upstream %s] Channel %s created at %s", b.GetUpstreamConn().Server, channel, createTime)

	b.SetChannelCreationTime(channel, createTime)

	// Forward message to client
	b.BroadcastToClients(msg)

	return nil
}
