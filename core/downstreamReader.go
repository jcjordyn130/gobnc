package core

import (
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/ergochat/irc-go/ircmsg"
	"github.com/ergochat/irc-go/ircreader"
	"github.com/rs/zerolog/log"
)

func (ds *DownstreamConnection) Init(b *Bouncer) {
	// Close connection at the end of the function
	defer log.Debug().Msgf("[downstream %s] Client disconnected", ds.Conn.RemoteAddr())

	// Setup a new reader for the downstream connection
	clientReader := ircreader.NewIRCReader(ds.Conn)

	// Start async client writer
	ds.StartAsyncClientWriter()

	// Start handshake
	err := b.newHandshake(*clientReader, ds)
	if err != nil {
		// Remove broken connection
		b.DisconnectDownstreamConnection(ds, "broken handshake")
		return
	}

	// If we aren't connected to the upstream server, let the client know!
	if b.GetUpstreamConn() == nil {
		fakeHostName := fmt.Sprintf("*status@%s", b.ServerName)

		rawmsg := ircmsg.MakeMessage(nil, fakeHostName, "NOTICE", "Disconnected from IRC!")
		msg, err := rawmsg.LineBytes()
		if err != nil {
			log.Debug().Msgf("[downstream %s] Error sending disconnected-to-upstream message: %v", ds.Conn.RemoteAddr(), err)
		}
		ds.Conn.Write(msg)
	}

	// Main downstream connection loop
	for {
		// Read raw bytes encoded line from client
		rawLine, err := clientReader.ReadLine()
		if err != nil {
			// Handle graceful EOF
			if err == io.EOF {
				log.Debug().Msgf("[downstream %s] Exiting connection loop due to EOF", ds.Conn.RemoteAddr())
				return
			}

			// Handle buffer overflow without a line terminator
			if err == ircreader.ErrReadQ {
				log.Debug().Msgf("[downstream %s] Forcing client to quit due to exceeding ReadQ", ds.Conn.RemoteAddr())
				b.DisconnectDownstreamConnection(ds, "Max ReadQ Exceeded")
				return
			}

			// Handle network errors
			if netErr, ok := errors.AsType[net.Error](err); ok {
				// We're going down
				if errors.Is(netErr, net.ErrClosed) {
					log.Debug().Msgf("[downstream %s] Exiting connection loop cleanly", ds.Conn.RemoteAddr())
					return
				}

				log.Debug().Msgf("[downstream %s] Exiting connection loop due to network error: %v", ds.Conn.RemoteAddr(), netErr)
				return
			}

			// Handle generic errors
			log.Debug().Msgf("[downstream %s] Error reading line: %v", ds.Conn.RemoteAddr(), err)
		}

		// Format line to an ircMessage and debug print
		line := string(rawLine)
		//log.Printf("[downstream %s] %s", conn.RemoteAddr(), line)
		msg, err := ircmsg.ParseLine(line)
		if err != nil {
			log.Debug().Msgf("[downstream %s] Error parsing line", ds.Conn.RemoteAddr())
			return
		}
		log.Debug().Msgf("[downstream %s] %+v", ds.Conn.RemoteAddr(), msg)

		// Call command handler
		err = b.Route(ds, msg)
		if HNFErr, ok := errors.AsType[*HandlerNotFound](err); ok && b.upstreamConn.Connected() {
			log.Error().Msgf("[downstream %s] Passing line to upstream: %v", ds.Conn.RemoteAddr(), HNFErr)
			b.upstreamConn.Send(string(rawLine))
		} else if _, ok := errors.AsType[*DownstreamClientQuitting](err); ok {
			log.Debug().Msgf("[downstream %s] Connection loop quitting due to clean break", ds.Conn.RemoteAddr())
			break
		} else if err != nil {
			log.Debug().Msgf("[downstream %s] Error calling command handler: %v", ds.Conn.RemoteAddr(), err)
		}
	}
}
