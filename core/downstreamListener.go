// core/downstreamListener.go
// This file containers the core connection loop for new downstream connections
package core

import (
	"context"
	"net"

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
		go downstreamConn.Init(b)
	}
}
