package downstreamHandlers

import (
	"github.com/rs/zerolog/log"

	"github.com/ergochat/irc-go/ircmsg"

	// Replace "yourmodule" with the module name defined in your go.mod
	"bouncer/core"
)

func HandleQUIT(b *core.Bouncer, ds *core.DownstreamConnection, msg ircmsg.Message) error {
	var quitMsg string

	if len(msg.Params) > 0 {
		quitMsg = msg.Params[0]
	} else {
		quitMsg = "default quit message"
	}

	log.Debug().Msgf("[downstream %s] QUIT received with message %s", ds.Conn.RemoteAddr(), quitMsg)

	b.DisconnectDownstreamConnection(ds, quitMsg)
	return nil
}
