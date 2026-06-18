// downstreamBot/isConnected.go
// This implements the "isConnected" command
package downstreambot

import (
	"strconv"

	"github.com/ergochat/irc-go/ircmsg"

	// Replace "yourmodule" with the module name defined in your go.mod
	"bouncer/core"
)

func handleIsConnected(b *core.Bouncer, ds *core.DownstreamConnection, msg ircmsg.Message) error {
	upstream := b.GetUpstreamConn()

	rawmsg := ircmsg.MakeMessage(nil, "*status", "PRIVMSG", "Upstream connected: "+strconv.FormatBool(upstream.Connected()))
	outmsg, err := rawmsg.LineBytes()
	if err != nil {
		return err
	}
	ds.Conn.Write(outmsg)

	return nil
}
