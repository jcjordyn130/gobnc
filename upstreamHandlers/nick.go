package upstreamHandlers

import (
	"github.com/rs/zerolog/log"

	"github.com/ergochat/irc-go/ircmsg"
)

func HandleNICK(b Router, msg ircmsg.Message) (bool, error) {
	upstream := b.GetUpstreamConn()

	if msg.Params[0] == upstream.CurrentNick() {
		log.Debug().Msgf("[upstream %s] Server forced NICK change to %s", b.GetUpstreamConn().Config.Server, msg.Params[0])

		// Update our downstream connection state
		b.ChangeDownstreamNick(upstream.CurrentNick())
	}

	return true, nil
}
