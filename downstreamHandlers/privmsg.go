package downstreamHandlers

import (
	"github.com/ergochat/irc-go/ircmsg"
	"github.com/rs/zerolog/log"

	// Replace "yourmodule" with the module name defined in your go.mod
	"bouncer/core"
	downstreambot "bouncer/downstreamBot"
)

func HandlePRIVMSG(b *core.Bouncer, ds *core.DownstreamConnection, msg ircmsg.Message) error {
	// Ignore blank messages
	if len(msg.Params) <= 0 {
		return nil
	}

	// Log to database
	if msg.Source == "" {
		log.Debug().Msgf("[downstream %s] Adding downstream nick to PRIVMSG command for log", ds.Conn.RemoteAddr())
		msg.Source = ds.Nick
		b.LogToDB(msg)
		msg.Source = ""
	} else {
		// I'm not sure if this is strictly *necessary* as I don't think IRC allows
		// clients to specify an arbitrary prefix.
		//
		// But better be safe than sorry.
		log.Debug().Msgf("[downstream %s] Downstream PRIVMSG already has source, logging bare", ds.Conn.RemoteAddr())
		b.LogToDB(msg)
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
