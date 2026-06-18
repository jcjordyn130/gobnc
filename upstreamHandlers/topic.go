package upstreamHandlers

import (
	"log"

	"github.com/ergochat/irc-go/ircmsg"
)

func HandleTOPIC(b Router, msg ircmsg.Message) error {
	channel := msg.Params[0]
	topic := msg.Params[1]

	log.Printf("[upstream %s] Setting topic for channel %s to: %s", b.GetUpstreamConn().Server, channel, topic)

	b.SetChannelTopic(channel, topic)

	// Forward message to client
	b.BroadcastToClients(msg)

	return nil
}
