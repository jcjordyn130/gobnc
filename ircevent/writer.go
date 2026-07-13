package ircevent

import (
	"time"

	"github.com/ergochat/irc-go/ircmsg"
	"github.com/rs/zerolog/log"
)

func (us *UpstreamConnection) writeLoop() {
	ticker := time.NewTicker(time.Duration(us.Config.WriterLoopTimeout) * time.Millisecond)

	go func() {
		// Ensure the ticker is properly stopped
		defer ticker.Stop()

		for {
			select {
			case msg, ok := <-us.msgChan:
				if !ok {
					us.logger.Debug().Msgf("Exiting AsyncClientWriter loop due to !ok on channel")
					us.writer.Flush()
					return
				}

				// Decode message
				ircmsgb, err := msg.LineBytes()
				if err != nil {
					us.logger.Warn().Msgf("error encoding message %+v", msg)
					continue
				}

				// Send message
				_, err = us.writer.Write(ircmsgb)
				us.logger.Debug().Msgf("[ASYNC] Sending message: %+v", msg)
				if err != nil {
					log.Error().Msgf("FATAL Error sending response: %v", err)
					if us.Cancel != nil {
						us.Cancel()
					}
					return
				}

			case <-ticker.C:
				// Periodically check if there is un-flushed data sitting in the buffer.
				if us.writer.Buffered() > 0 {
					us.logger.Debug().Msgf("Flushing buffered data to client")
					err := us.writer.Flush() // Flush the buffered data to the underlying connection
					if err != nil {
						us.logger.Error().Msgf("Error flushing writer: %v", err)
						if us.Cancel != nil {
							us.Cancel()
						}
						return
					}
				}
			}
		}
	}()
}

func (us *UpstreamConnection) WriteMsg(msg ircmsg.Message) {
	us.logger.Trace().Msgf("Writing message %+v", msg)
	us.msgChan <- msg
}

func (us *UpstreamConnection) WriteMsgNoQ(msg ircmsg.Message) {
	// WARN: unsafe to use concurrently
	us.logger.Trace().Msgf("Writing message (NO QUEUE) %+v", msg)

	// Decode message
	ircmsgb, err := msg.LineBytes()
	if err != nil {
		us.logger.Error().Msgf("error encoding message %+v", msg)
		panic("failed handshake")
	}

	// Send message
	_, err = us.writer.Write(ircmsgb)
	us.logger.Debug().Msgf("[NOQUEUE] Sending message: %+v", msg)
	if err != nil {
		log.Error().Msgf("FATAL Error sending response: %v", err)
		if us.Cancel != nil {
			us.Cancel()
		}
		return
	}
}

// Capability HACK
// TODO: refactor
func (us *UpstreamConnection) SendIRCMessage(msg ircmsg.Message) error {
	us.WriteMsg(msg)
	return nil
}
