package ircevent

import "github.com/ergochat/irc-go/ircmsg"

// List of IRCv3 capabilities that we support
var supportedCaps = map[string]bool{
	"server-time":  true,
	"echo-message": true,
	"multi-prefix": true,
}

func (us *UpstreamConnection) sendHandshake() {
	us.sendCapNeg()
}

func (us *UpstreamConnection) sendCapNeg() {
	us.WriteMsg(ircmsg.MakeMessage(nil, "", "CAP LS"))
}
