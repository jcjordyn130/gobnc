package downstreamHandlers

import (
	"github.com/ergochat/irc-go/ircmsg"
	"github.com/rs/zerolog/log"

	// Replace "yourmodule" with the module name defined in your go.mod
	"bouncer/core"
)

func HandleAWAY(b *core.Bouncer, ds *core.DownstreamConnection, msg ircmsg.Message) error {
	if len(msg.Params) > 0 {
		log.Debug().Msgf("[downstream %s] User setting away for reason %s", ds.Conn.RemoteAddr(), msg.Params[0])
	} else {
		log.Debug().Msgf("[downstream %s] User unsetting away", ds.Conn.RemoteAddr())
	}

	b.GetUpstreamConn().SendIRCMessage(msg)
	return nil
}
