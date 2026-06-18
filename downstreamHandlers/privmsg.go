package downstreamHandlers

import (
	"github.com/ergochat/irc-go/ircmsg"

	// Replace "yourmodule" with the module name defined in your go.mod
	"bouncer/core"
	downstreambot "bouncer/downstreamBot"
)

func HandlePRIVMSG(b *core.Bouncer, ds *core.DownstreamConnection, msg ircmsg.Message) error {
	// Ignore blank messages
	if len(msg.Params) <= 0 {
		return nil
	}

	// Handle bouncer commands
	if msg.Params[0] == "*status" {
		return downstreambot.HandlePRIVMSG(b, ds, msg)
	}

	// Handle self-messages
	b.EchoToOtherClients(ds, msg)

	// Pass to upstream serv
	upstream := b.GetUpstreamConn()
	upstream.SendIRCMessage(msg)

	return nil
}
