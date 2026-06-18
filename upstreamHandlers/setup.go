package upstreamHandlers

import (
	"log"

	"github.com/ergochat/irc-go/ircevent"
	"github.com/ergochat/irc-go/ircmsg"
)

// Router defines the abilities the upstream handlers need
// from the core Bouncer to do their job.
type Router interface {
	//SendToClient(conn net.Conn, message ircmsg.Message)
	GetUpstreamConn() *ircevent.Connection
	BroadcastToClients(msg ircmsg.Message)
	LogToDB(msg ircmsg.Message)
	ChangeDownstreamNick(newnick string)
	SetChannelTopic(channel, topic string)
	AddChannelUsers(channel string, users []string)
	IsJoined(channel string) bool
	DeleteChannel(channel string)
	RemoveUserFromChannel(channel string, user string)
	ApplyModes(channel string, modeStr string, args []string)
	SetChannelCreationTime(channel string, createTime string)
	SetChannelMode(channel string, modeStr []string)
	OnUpstreamJoin(channelName string)
	ClearMOTD()
	CacheMOTD(msg ircmsg.Message)
}

type UpstreamCommandHandler func(b Router, msg ircmsg.Message) error

// Register attaches all your upstream callbacks to a new connection.
// It accepts the interface, NOT the core.Bouncer struct.
func Register(b Router, cmd string, callback UpstreamCommandHandler) {
	upstream := b.GetUpstreamConn()

	upstream.AddCallback(cmd, func(e ircmsg.Message) {
		log.Printf("[upstream %s] Routing to command handler for: %s", upstream.Server, cmd)
		callback(b, e)
	})
}
