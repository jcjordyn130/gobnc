package core

import (
	"bufio"
	"strings"
	"time"

	"github.com/ergochat/irc-go/ircmsg"
	"github.com/rs/zerolog/log"
)

func (ds *DownstreamConnection) StartAsyncClientWriter() {
	ds.msgChan = make(chan []byte)
	writer := bufio.NewWriter(ds.Conn)

	ticker := time.NewTicker(75 * time.Millisecond)

	go func() {
		// Ensure the ticker is properly stopped
		defer ticker.Stop()

		for {
			select {
			case ircmsgb, ok := <-ds.msgChan:
				if !ok {
					log.Debug().Msgf("[downstream %s] Exiting AsyncClientWriter loop due to !ok on channel", ds.Conn.RemoteAddr())
					writer.Flush()
					return
				}

				// Send message
				_, err := writer.Write(ircmsgb)
				log.Debug().Msgf("[downstream %s] [ASYNC] Sending message: %s", ds.Conn.RemoteAddr(), strings.TrimSpace(string(ircmsgb)))
				if err != nil {
					log.Error().Msgf("[downstream %s] FATAL Error sending response: %v", ds.Conn.RemoteAddr(), err)
					if ds.Cancel != nil {
						ds.Cancel()
					}
					return
				}

			case <-ticker.C:
				// Periodically check if there is un-flushed data sitting in the buffer.
				if writer.Buffered() > 0 {
					log.Debug().Msgf("[downstream %s] Flushing buffered data to client", ds.Conn.RemoteAddr())
					err := writer.Flush() // Flush the buffered data to the underlying connection
					if err != nil {
						log.Error().Msgf("[downstream %s] Error flushing writer: %v", ds.Conn.RemoteAddr(), err)
						if ds.Cancel != nil {
							ds.Cancel()
						}
						return
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

	// Serialize message
	msgBytes, err := ircmsg.LineBytes()
	if err != nil {
		return err
	}

	ds.msgChan <- msgBytes
	return nil
}
