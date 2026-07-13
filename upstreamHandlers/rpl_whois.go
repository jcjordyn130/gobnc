package upstreamHandlers

import (
	"bouncer/models"

	"github.com/ergochat/irc-go/ircmsg"
	"github.com/rs/zerolog/log"
)

func Handle311(b Router, msg ircmsg.Message) (bool, error) {
	// Forward message to client
	b.BroadcastToClients(msg)
	return true, nil
}

func Handle319(b Router, msg ircmsg.Message) (bool, error) {
	// Forward message to client
	b.BroadcastToClients(msg)
	return true, nil
}

func Handle312(b Router, msg ircmsg.Message) (bool, error) {
	// Forward message to client
	b.BroadcastToClients(msg)
	return true, nil
}

func Handle671(b Router, msg ircmsg.Message) (bool, error) {
	// Forward message to client
	b.BroadcastToClients(msg)
	return true, nil
}

func Handle338(b Router, msg ircmsg.Message) (bool, error) {
	// Forward message to client
	b.BroadcastToClients(msg)
	return true, nil
}

func Handle317(b Router, msg ircmsg.Message) (bool, error) {
	// Forward message to client
	b.BroadcastToClients(msg)
	return true, nil
}

func Handle318(b Router, msg ircmsg.Message) (bool, error) {
	// Forward message to client
	b.BroadcastToClients(msg)
	return true, nil
}

func Handle330(b Router, msg ircmsg.Message) (bool, error) {
	// Forward message to client
	b.BroadcastToClients(msg)
	return true, nil
}

func Handle402(b Router, msg ircmsg.Message) (bool, error) {
	// Forward message to client
	b.BroadcastToClients(msg)
	return true, nil
}

func Handle301(b Router, msg ircmsg.Message) (bool, error) {
	// We have an away message
	// AFAIK the IRC protocol doesn't allow away without a message,
	// at least in the WHOIS output.
	if len(msg.Params) > 2 {
		nick := msg.Params[1]
		awayMsg := msg.Params[2]
		log.Trace().Msgf("[upstream %s] Away message for %s receieved from WHOIS", b.GetUpstreamConn().Config.Server, nick)

		b.ModifyUser(nick, func(u *models.UserState) {
			u.Away = true
			u.AwayMessage = awayMsg
		})
	}

	// Forward message to client
	b.BroadcastToClients(msg)
	return true, nil
}
