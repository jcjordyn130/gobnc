package core

import (
	"github.com/ergochat/irc-go/ircmsg"
	"github.com/rs/zerolog/log"
)

func (ds *DownstreamConnection) StartAsyncClientWriter() {
	ds.msgChan = make(chan ircmsg.Message)

	go func() {
		for {
			// Get message from channel
			msg, ok := <-ds.msgChan
			if !ok {
				log.Debug().Msgf("[downstream %s] Exiting AsyncClientWriter loop due to !ok on channel", ds.Conn.RemoteAddr())
				break
			}

			// Get message
			ircmsgb, err := msg.LineBytes()
			if err != nil {
				log.Error().Msgf("[downstream %s] Error sending response: %v", ds.Conn.RemoteAddr(), err)
				continue
			}

			// Send message
			_, err = ds.Conn.Write(ircmsgb)
			log.Debug().Msgf("[downstream %s] [ASYNC] Sending message: %v", ds.Conn.RemoteAddr(), msg)
			if err != nil {
				log.Error().Msgf("[downstream %s] Error sending response: %v", ds.Conn.RemoteAddr(), err)
				continue
			}
		}
	}()
}

func (ds *DownstreamConnection) SendToClient(ircmsg ircmsg.Message) error {
	if ds.msgChan == nil {
		panic("SendToClient attempted without asyncWriter started!")
	}

	ds.msgChan <- ircmsg
	return nil
}
