// core/downstreamListener.go
// This file containers the core connection loop for new downstream connections
package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/ergochat/irc-go/ircmsg"
	"github.com/ergochat/irc-go/ircreader"
	"github.com/rs/zerolog/log"
)

func (b *Bouncer) ListenDownstream(ctx context.Context, bindAddress string) {
	log.Debug().Msgf("Starting downstream accept loop for bindAddress %s", bindAddress)
	listener, err := net.Listen("tcp", bindAddress)
	if err != nil {
		log.Fatal().Msgf("Failed to bind downstream port: %v", err)
	}

	log.Debug().Msgf("Downstream listening on %s", bindAddress)

	// Start background goroutine to listen for context cancel
	go func() {
		<-ctx.Done()
		log.Info().Msg("Context cancelled, closing downstream listener...")
		listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				log.Debug().Msg("Exiting downstream accept loop due to context cancel")
				return
			}

			log.Debug().Msgf("Downstream accept error: %v", err)
			continue
		}

		log.Debug().Msgf("[downstream %s] Client connected", conn.RemoteAddr())
		log.Debug().Msgf("Client attached to bouncer %s", conn.RemoteAddr())

		// Add context state
		clientCtx, cancel := context.WithCancel(ctx)

		downstreamConn := &DownstreamConnection{
			Conn:              conn,
			Ctx:               clientCtx,
			Cancel:            cancel,
			Caps:              make(map[string]bool),
			HandshakeComplete: false,
		}

		b.AddDownstreamConnection(downstreamConn)

		// Spawn a dedicated goroutine for each client connection
		go b.handleClient(downstreamConn)
	}
}

func (b *Bouncer) handleClient(ds *DownstreamConnection) {
	// Close connection at the end of the function
	defer log.Debug().Msgf("[downstream %s] Client disconnected", ds.Conn.RemoteAddr())
	defer b.DisconnectDownstreamConnection(ds, "Graceful")

	// Setup a new reader for the downstream connection
	clientReader := ircreader.NewIRCReader(ds.Conn)

	// Start async client writer
	b.StartAsyncClientWriter(ds)

	// Start handshake
	b.handleHandshake(*clientReader, ds)

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
			log.Debug().Msgf("[downstream %s] Passing line to upstream: %v", ds.Conn.RemoteAddr(), HNFErr)
			b.upstreamConn.Send(string(rawLine))
		} else if _, ok := errors.AsType[*DownstreamClientQuitting](err); ok {
			log.Debug().Msgf("[downstream %s] Connection loop quitting due to clean break", ds.Conn.RemoteAddr())
			break
		} else if err != nil {
			log.Debug().Msgf("[downstream %s] Error calling command handler: %v", ds.Conn.RemoteAddr(), err)
		}
	}
}
