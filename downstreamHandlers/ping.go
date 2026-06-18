package downstreamHandlers

import (
	"github.com/ergochat/irc-go/ircmsg"

	// Replace "yourmodule" with the module name defined in your go.mod
	"bouncer/core"
)

func HandlePING(b *core.Bouncer, ds *core.DownstreamConnection, msg ircmsg.Message) error {
	token := msg.Params[0]

	// You now have full access to b.UpstreamConn!
	pong := ircmsg.MakeMessage(nil, b.ServerName, "PONG", b.ServerName, token)

	b.SendToClient(ds, pong)
	return nil
}
