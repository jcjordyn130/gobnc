package upstreamHandlers

import (
	"log"

	"github.com/ergochat/irc-go/ircmsg"
)

func HandleNICK(b Router, msg ircmsg.Message) error {
	upstream := b.GetUpstreamConn()

	if msg.Params[0] == upstream.CurrentNick() {
		log.Printf("[upstream %s] Server forced NICK change to %s", upstream.Server, msg.Params[0])

		// Update our downstream connection state
		b.ChangeDownstreamNick(upstream.CurrentNick())
	}

	return nil
}
