package upstreamHandlers

import (
	"bouncer/ircevent"

	"github.com/ergochat/irc-go/ircmsg"
)

func HandlePING(b Router, us *ircevent.UpstreamConnection, msg ircmsg.Message) (bool, error) {
	// You now have full access to b.UpstreamConn!
	pong := ircmsg.MakeMessage(nil, "", "PONG", msg.Params...)
	b.GetUpstreamConn().WriteMsg(pong)
	//if err != nil {
	//	return err
	//}

	return true, nil
}
