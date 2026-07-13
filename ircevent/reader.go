package ircevent

import (
	"errors"
	"io"
	"net"

	"github.com/ergochat/irc-go/ircmsg"
	"github.com/ergochat/irc-go/ircreader"
)

func (us *UpstreamConnection) readLoop() {
	for {
		// Read raw bytes encoded line from client
		rawLine, err := us.reader.ReadLine()
		if err != nil {
			// Handle graceful EOF
			if err == io.EOF {
				us.logger.Debug().Msgf("Exiting connection loop due to EOF")
				return
			}

			// Handle buffer overflow without a line terminator
			if err == ircreader.ErrReadQ {
				us.logger.Debug().Msgf("Forcing client to quit due to exceeding ReadQ")
				// TODO: force quit
				return
			}

			// Handle network errors
			if netErr, ok := errors.AsType[net.Error](err); ok {
				// We're going down
				if errors.Is(netErr, net.ErrClosed) {
					us.logger.Debug().Msgf("Exiting connection loop cleanly")
					return
				}

				us.logger.Debug().Msgf("Exiting connection loop due to network error: %v", netErr)
				return
			}

			// Handle generic errors
			us.logger.Debug().Msgf("Error reading line: %v", err)
		}

		// Format line to an ircMessage and debug print
		line := string(rawLine)
		//us.logger.Printf("%s", conn.RemoteAddr(), line)
		msg, err := ircmsg.ParseLine(line)
		if err != nil {
			us.logger.Debug().Msgf("Error parsing line")
			return
		}
		us.logger.Debug().Msgf("%+v", msg)

		// Handle callbacks
		if !us.connected {
			us.handleHandshake(msg)
		} else {
			go us.handleCallback(msg)
		}
	}
}
