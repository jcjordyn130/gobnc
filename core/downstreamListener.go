// core/downstreamListener.go
// This file containers the core connection loop for new downstream connections
package core

import (
	"errors"
	"io"
	"log"
	"net"

	"github.com/ergochat/irc-go/ircmsg"
	"github.com/ergochat/irc-go/ircreader"
)

func (b *Bouncer) handleClient(ds *DownstreamConnection) {
	// Close connection at the end of the function
	defer log.Printf("[downstream %s] Client disconnected", ds.Conn.RemoteAddr())
	defer ds.Conn.Close()

	// Setup a new reader for the downstream connection
	clientReader := ircreader.NewIRCReader(ds.Conn)

	// Start handshake
	b.handleHandshake(*clientReader, ds)

	// Send message history
	// TODO: how to send DM history?
	//b.SendHistory(ds)

	// If we aren't connected to the upstream server, let the client know!
	rawmsg := ircmsg.MakeMessage(nil, "*status@jordynsblog.org", "NOTICE", "Disconnected from IRC!")
	msg, err := rawmsg.LineBytes()
	if err != nil {
		log.Printf("[downstream %s] Error sending disconnected-to-upstream message: %v", ds.Conn.RemoteAddr(), err)
	}
	ds.Conn.Write(msg)

	// Main downstream connection loop
	for {
		// Read raw bytes encoded line from client
		rawLine, err := clientReader.ReadLine()
		if err != nil {
			// Handle graceful EOF
			if err == io.EOF {
				log.Printf("[downstream %s] Exiting connection loop due to EOF", ds.Conn.RemoteAddr())
				return
			}

			// Handle buffer overflow without a line terminator
			// TODO: maybe send an actual QUIT instead of a connection close?
			if err == ircreader.ErrReadQ {
				log.Printf("[downstream %s] Forcing client to quit due to exceeding ReadQ", ds.Conn.RemoteAddr())
				return
			}

			// Handle network errors
			if netErr, ok := errors.AsType[net.Error](err); ok {
				// We're going down
				if errors.Is(netErr, net.ErrClosed) {
					log.Printf("[downstream %s] Exiting connection loop cleanly", ds.Conn.RemoteAddr())
					return
				}

				log.Printf("[downstream %s] Exiting connection loop due to network error: %v", ds.Conn.RemoteAddr(), netErr)
				return
			}

			// Handle generic errors
			log.Printf("[downstream %s] Error reading line: %v", ds.Conn.RemoteAddr(), err)
		}

		// Format line to an ircMessage and debug print
		line := string(rawLine)
		//log.Printf("[downstream %s] %s", conn.RemoteAddr(), line)
		msg, err := ircmsg.ParseLine(line)
		if err != nil {
			log.Printf("[downstream %s] Error parsing line", ds.Conn.RemoteAddr())
			return
		}
		log.Printf("[downstream %s] %+v", ds.Conn.RemoteAddr(), msg)

		// Call command handler
		err = b.Route(ds, msg)
		if HNFErr, ok := errors.AsType[*HandlerNotFound](err); ok && b.upstreamConn.Connected() {
			log.Printf("[downstream %s] Passing line to upstream: %v", ds.Conn.RemoteAddr(), HNFErr)
			b.upstreamConn.Send(string(rawLine))
		} else if _, ok := errors.AsType[*DownstreamClientQuitting](err); ok {
			log.Printf("[downstream %s] Connection loop quitting due to clean break", ds.Conn.RemoteAddr())
			break
		} else if err != nil {
			log.Printf("[downstream %s] Error calling command handler: %v", ds.Conn.RemoteAddr(), err)
		}
	}
}
