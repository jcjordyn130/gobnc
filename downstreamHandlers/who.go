package downstreamHandlers

import (
	"github.com/ergochat/irc-go/ircmsg"

	// Replace "yourmodule" with the module name defined in your go.mod
	"bouncer/core"
)

func DisConnHandleWHO(b *core.Bouncer, ds *core.DownstreamConnection, msg ircmsg.Message) error {
	respNick := msg.Params[0]

	// Send IRC 352 Message (WHO)
	respMsg := ircmsg.MakeMessage(
		nil,                   // Tags
		"bnc.jordynsblog.org", // Prefix (The server claiming to send the reply)
		"352",                 // Command

		// --- The 352 Parameters ---
		ds.Nick,               // [0] The client receiving the numeric
		"*",                   // [1] The channel being queried
		respNick,              // [2] User/Ident
		"bnc.jordynsblog.org", // [3] Hostname
		"bnc.jordynsblog.org", // [4] Server the user is connected to
		respNick,              // [5] The Nickname (e.g., canonical "Trifton")
		"H",                   // [6] Flags (H = Here, G = Gone, + = Voice, @ = Op)
		"0 "+"Ghost User",     // [7] Hopcount + Real Name (Must be single string)
	)
	respMsgBytes, err := respMsg.LineBytes()
	if err != nil {
		return err
	}

	ds.Conn.Write(respMsgBytes)

	// Send IRC 315 Message (END WHO)
	endMsg := ircmsg.MakeMessage(
		nil,
		"bnc.jordynsblog.org",
		"315",
		respNick,
		"*",
		"End of /WHO list.",
	)
	endMsgBytes, err := endMsg.LineBytes()
	if err != nil {
		return err
	}

	ds.Conn.Write(endMsgBytes)

	return nil
}

func HandleWHO(b *core.Bouncer, ds *core.DownstreamConnection, msg ircmsg.Message) error {
	upstream := b.GetUpstreamConn()

	// Handle WHO replies when we have no upstream
	if !upstream.Connected() {
		return DisConnHandleWHO(b, ds, msg)
	} else {
		// TODO: implement
		return nil
	}
}
