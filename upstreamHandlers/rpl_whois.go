package upstreamHandlers

import (
	"github.com/ergochat/irc-go/ircmsg"
)

func Handle311(b Router, msg ircmsg.Message) error {
	// Forward message to client
	b.BroadcastToClients(msg)
	return nil
}

func Handle319(b Router, msg ircmsg.Message) error {
	// Forward message to client
	b.BroadcastToClients(msg)
	return nil
}

func Handle312(b Router, msg ircmsg.Message) error {
	// Forward message to client
	b.BroadcastToClients(msg)
	return nil
}

func Handle671(b Router, msg ircmsg.Message) error {
	// Forward message to client
	b.BroadcastToClients(msg)
	return nil
}

func Handle338(b Router, msg ircmsg.Message) error {
	// Forward message to client
	b.BroadcastToClients(msg)
	return nil
}

func Handle317(b Router, msg ircmsg.Message) error {
	// Forward message to client
	b.BroadcastToClients(msg)
	return nil
}

func Handle318(b Router, msg ircmsg.Message) error {
	// Forward message to client
	b.BroadcastToClients(msg)
	return nil
}
