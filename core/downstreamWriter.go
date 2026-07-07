package core

import (
	"bufio"
	"time"

	"github.com/ergochat/irc-go/ircmsg"
	"github.com/rs/zerolog/log"
)

func (ds *DownstreamConnection) StartAsyncClientWriter() {
	ds.msgChan = make(chan ircmsg.Message)
	writer := bufio.NewWriter(ds.Conn)

	ticker := time.NewTicker(75 * time.Millisecond)

	go func() {
		for {
			select {
			case msg, ok := <-ds.msgChan:
				if !ok {
					log.Debug().Msgf("[downstream %s] Exiting AsyncClientWriter loop due to !ok on channel", ds.Conn.RemoteAddr())
					writer.Flush()
					return
				}

				// Get message
				ircmsgb, err := msg.LineBytes()
				if err != nil {
					log.Error().Msgf("[downstream %s] Error sending response: %v", ds.Conn.RemoteAddr(), err)
					continue
				}

				// Send message
				_, err = writer.Write(ircmsgb)
				log.Debug().Msgf("[downstream %s] [ASYNC] Sending message: %v", ds.Conn.RemoteAddr(), msg)
				if err != nil {
					log.Error().Msgf("[downstream %s] Error sending response: %v", ds.Conn.RemoteAddr(), err)
					continue
				}

			case <-ticker.C:
				// Periodically check if there is un-flushed data sitting in the buffer.
				if writer.Buffered() > 0 {
					log.Debug().Msgf("[downstream %s] Flushing buffered data to client", ds.Conn.RemoteAddr())
					err := writer.Flush() // Flush the buffered data to the underlying connection
					if err != nil {
						log.Error().Msgf("[downstream %s] Error flushing writer: %v", ds.Conn.RemoteAddr(), err)
					}
				}
			}
		}
	}()
}

func (ds *DownstreamConnection) Close() {
	log.Debug().Msgf("[downstream %s] Closing downstream connection", ds.Conn.RemoteAddr())
	if ds.msgChan != nil {
		close(ds.msgChan)
		ds.msgChan = nil
	}
}

func (ds *DownstreamConnection) SendToClient(ircmsg ircmsg.Message) error {
	if ds.msgChan == nil {
		panic("SendToClient attempted without asyncWriter started!")
	}

	ds.msgChan <- ircmsg
	return nil
}
