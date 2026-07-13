package upstreamHandlers

import (
	"bouncer/ircevent"
	"bouncer/models"

	"github.com/ergochat/irc-go/ircmsg"
)

// Router defines the abilities the upstream handlers need
// from the core Bouncer to do their job.
type Router interface {
	GetUpstreamConn() *ircevent.UpstreamConnection
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
	JoinAutoJoinChannels() error
	RemoveUserFromAllChannels(nick string)
	ModifyUser(nick string, modifier func(user *models.UserState))
}

type UpstreamCommandHandler func(b Router, msg ircmsg.Message) (bool, error)

// Register attaches all your upstream callbacks to a new connection.
// It accepts the interface, NOT the core.Bouncer struct.
func Register(b Router, cmd string, callback UpstreamCommandHandler) {
	upstream := b.GetUpstreamConn()

	upstream.RegisterCallback(cmd, func(e ircmsg.Message) (bool, error) {
		callback(b, e)
		return true, nil
	})
}
