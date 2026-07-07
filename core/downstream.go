package core

import "github.com/ergochat/irc-go/ircmsg"

func (ds *DownstreamConnection) SendToClient(ircmsg ircmsg.Message) error {
	if ds.msgChan == nil {
		panic("SendToClient attempted without asyncWriter started!")
	}

	ds.msgChan <- ircmsg
	return nil
}
