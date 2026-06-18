package upstreamHandlers

import (
	"github.com/ergochat/irc-go/ircmsg"
)

func HandlePING(b Router, msg ircmsg.Message) error {
	// You now have full access to b.UpstreamConn!
	pong := ircmsg.MakeMessage(nil, "", "PONG", msg.Params...)
	err := b.GetUpstreamConn().SendIRCMessage(pong)
	if err != nil {
		return err
	}

	return nil
}
