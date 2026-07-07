package downstreamHandlers

import (
	"strings"

	"bouncer/core"

	"github.com/ergochat/irc-go/ircmsg"
)

func DisConHandleUSERHOST(b *core.Bouncer, ds *core.DownstreamConnection, msg ircmsg.Message) error {
	// A 302 RPL_USERHOST has two parameters:
	// [0] Client's Nickname
	// [1] Space-separated list of replies
	var replies []string

	// 1. Look up each requested nickname in your internal state
	for _, reqNick := range msg.Params {
		// 2. Format the individual reply string
		// Format: nickname[*]=user@host

		// '+' indicates active, '-' indicates the user is marked as away
		// '*' indicates the user is an IRC operator
		// Users are NEVER away with a disconnected upstream server
		// BNC users are IRC operators when not connected

		// Construct the segment (e.g., "Trifton*=+user@host.com")
		segment := reqNick + "*=+" + "GhOsT" + "@" + b.ServerName
		replies = append(replies, segment)
	}

	// 3. Join all individual segments into a single space-separated string
	trailingParam := strings.Join(replies, " ")

	// 4. Synthesize the 302 numeric
	msg = ircmsg.MakeMessage(
		nil,           // Tags (nil for standard numerics)
		b.ServerName,  // Prefix (The bouncer claiming to be the server)
		"302",         // RPL_USERHOST command
		ds.Nick,       // [0] The client receiving the response
		trailingParam, // [1] The packed string of results
	)

	// 5. Serialize to raw bytes and send down the socket
	line, err := msg.LineBytes()
	if err != nil {
		return err
	}

	ds.Conn.Write(line)
	return nil
}

func HandleUSERHOST(b *core.Bouncer, ds *core.DownstreamConnection, msg ircmsg.Message) error {
	// Ignore empty requests
	if len(msg.Params) == 0 {
		return nil
	}

	upstream := b.GetUpstreamConn()

	// Handle USERHOST
	if !upstream.Connected() {
		return DisConHandleUSERHOST(b, ds, msg)
	} else {
		// Maybe this needs an upstream handler, but for now just echo to upstream
		return b.GetUpstreamConn().SendIRCMessage(msg)
	}
}
