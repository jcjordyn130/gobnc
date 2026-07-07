// core/handshake.go
// This file only contains the function for an IRC handshake (USER, NICK, and cap negotation)
//
// I split this off into its own file because of how long the function is
// TODO: don't use raw IRC messages
package core

import (
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/ergochat/irc-go/ircmsg"
	"github.com/ergochat/irc-go/ircreader"
	"github.com/rs/zerolog/log"
)

func (b *Bouncer) handleHandshake(reader ircreader.Reader, ds *DownstreamConnection) error {
	log.Debug().Msgf("[downstream %s] Starting capability negoiation!", ds.Conn.RemoteAddr())
	capNegActive := true

	for {
		// read raw line from handshake
		rawLine, err := reader.ReadLine()
		if err != nil {
			log.Debug().Msgf("[downstream %s] handshake error: %v", ds.Conn.RemoteAddr(), err)
			return err
		}

		// parse line
		line := string(rawLine)
		msg, err := ircmsg.ParseLine(line)
		if err != nil {
			log.Debug().Msgf("[downstream %s] handshake error: %v", ds.Conn.RemoteAddr(), err)
			return err
		}

		log.Debug().Msgf("[downstream %s] %+v", ds.Conn.RemoteAddr(), msg)

		switch msg.Command {
		case "CAP":
			// Check for malformed CAP
			if len(msg.Params) == 0 {
				log.Debug().Msgf("[downstream %s] zero-length CAP received???", ds.Conn.RemoteAddr())
				continue
			}

			subCommand := msg.Params[0]

			if subCommand == "LS" {
				log.Debug().Msgf("[downstream %s] client is asking for cap list", ds.Conn.RemoteAddr())
				capNegActive = true

				// 2. Extract the keys from the map
				var availableCaps []string
				for capName, isSupported := range supportedCaps {
					if isSupported {
						availableCaps = append(availableCaps, capName)
					}
				}

				// 3. Join them into a space-separated string
				lsString := strings.Join(availableCaps, " ")

				capMsg := ircmsg.MakeMessage(nil, b.ServerName, "CAP", "*", "LS", lsString)
				log.Debug().Msgf("[downstream %s] Sending CAP LS: %s", ds.Conn.RemoteAddr(), lsString)
				b.SendToClient(ds, capMsg)
			} else if subCommand == "REQ" && len(msg.Params) > 1 {
				requestedCaps := msg.Params[1]

				var ackedCaps []string // Asked for and we support
				var nakedCaps []string // Asked for and we do NOT support

				// Parse everything the client asked for
				for requestedCap := range strings.SplitSeq(requestedCaps, " ") {
					capName := strings.TrimSpace(requestedCap)
					if capName == "" {
						continue
					}

					// Check if our bouncer actually supports it
					if supportedCaps[capName] {
						log.Debug().Msgf("[downstream %s] ACKing capability: %s", ds.Conn.RemoteAddr(), capName)
						ds.Caps[capName] = true
						ackedCaps = append(ackedCaps, capName)
					} else {
						log.Debug().Msgf("[downstream %s] Rejecting unknown capability: %s", ds.Conn.RemoteAddr(), capName)
						nakedCaps = append(nakedCaps, capName)
					}
				}

				// IRCv3 protocol technically *supports* one at a time, but it suggests batch
				// Batch send the ACKs
				if len(ackedCaps) > 0 {
					ackStr := strings.Join(ackedCaps, " ")
					capMsg := ircmsg.MakeMessage(nil, b.ServerName, "CAP", "*", "ACK", ackStr)
					b.SendToClient(ds, capMsg)
				}

				// Batch send the NAKs (declined capabilities)
				if len(nakedCaps) > 0 {
					nakStr := strings.Join(nakedCaps, " ")
					capMsg := ircmsg.MakeMessage(nil, b.ServerName, "CAP", "*", "NAK", nakStr)
					b.SendToClient(ds, capMsg)
				}
			} else if subCommand == "END" {
				log.Debug().Msgf("[downstream %s] cap negotiation end", ds.Conn.RemoteAddr())
				capNegActive = false
			}

		case "NICK":
			if len(msg.Params) > 0 {
				log.Debug().Msgf("[downstream %s] setting nickname to %s", ds.Conn.RemoteAddr(), msg.Params[0])
				ds.Nick = msg.Params[0]
			}

		case "USER":
			if len(msg.Params) >= 4 {
				// TODO: maybe we should store the username and realname somewhere? idk
				log.Debug().Msgf("[downstream %s] received USER command %s", ds.Conn.RemoteAddr(), msg.Params[0])
			}
		}

		// 2. The Golden Rule Exit Check
		// This must live OUTSIDE the switch so it can safely break the FOR loop.
		if ds.Nick != "" && !capNegActive {
			break
		}
	}

	// Send RPL_001 WELCOME
	if b.upstreamConn.Connected() {
		log.Debug().Msgf("[downstream %s] sending upstream nick %s to client", ds.Conn.RemoteAddr(), b.upstreamConn.CurrentNick())
		rplWelcome := ircmsg.MakeMessage(nil, b.ServerName, "001", b.upstreamConn.CurrentNick(), "Welcome to the Golang BNC!")
		ds.Nick = b.upstreamConn.CurrentNick()
		b.SendToClient(ds, rplWelcome)
	} else {
		log.Debug().Msgf("[downstream %s] sending client nick %s to client due to no connection!", ds.Conn.RemoteAddr(), ds.Nick)
		rplWelcome := ircmsg.MakeMessage(nil, b.ServerName, "001", ds.Nick, "Welcome to the Golang BNC!")
		b.SendToClient(ds, rplWelcome)
	}

	// Set connection state to handshake complete
	ds.HandshakeComplete = true

	// Send MOTD
	b.SendCachedMOTD(ds)

	// Send current upstream nick
	// Not *technically* needed but some older clients don't listen to 001
	// and it makes the handshake code cleaner
	//if b.upstreamConn.Connected() {
	//	log.Debug().Msgf("[downstream %s] Changing nick from client given %s to upstream %s", ds.Conn.RemoteAddr(), ds.Nick, b.upstreamConn.CurrentNick())
	//	b.ChangeDownstreamNick(b.upstreamConn.CurrentNick())
	//} else {
	//	log.Debug().Msgf("[downstream %s] Not sending upstream nick as we aren't connected!", ds.Conn.RemoteAddr())
	//}

	// Send channel joins
	go b.sendJoinedChannels(ds)

	// Send open queries
	go b.sendOpenQueries(ds)

	log.Debug().Msgf("[downstream %s] returning to main client loop", ds.Conn.RemoteAddr())
	return nil
}

func (b *Bouncer) sendJoinedChannels(ds *DownstreamConnection) {
	prefix := fmt.Sprintf("%s!GHoSt@%s", ds.Nick, b.ServerName)

	// Duplicate the channels map to avoid holding up the client loop
	b.mu.Lock()
	channels := maps.Clone(b.Channels)
	b.mu.Unlock()

	for _, chanState := range channels {
		log.Debug().Msgf("[downstream %s] Sending JOIN for %s", ds.Conn.RemoteAddr(), chanState.Name)

		joinMsg := ircmsg.MakeMessage(nil, prefix, "JOIN", chanState.Name)
		b.SendToClient(ds, joinMsg)

		// 2. The Topic (332)
		if chanState.Topic != "" {
			b.SendToClient(ds, ircmsg.MakeMessage(nil, b.ServerName, "332", ds.Nick, chanState.Name, chanState.Topic))
		}

		// 3. Channel Modes (324)
		if chanState.Modes != "" {
			b.SendToClient(ds, ircmsg.MakeMessage(nil, b.ServerName, "324", ds.Nick, chanState.Name, chanState.Modes))
		}

		// 4. Creation Time (329)
		if chanState.CreationTime != "" {
			b.SendToClient(ds, ircmsg.MakeMessage(nil, b.ServerName, "329", ds.Nick, chanState.Name, chanState.CreationTime))
		}

		// Send /NAMES list
		b.sendNamesList(ds, chanState)

		// Send history
		go b.SendHistory(&chanState.Name, ds)
	}
}

func (b *Bouncer) sendOpenQueries(ds *DownstreamConnection) {
	log.Debug().Msgf("[downstream %s] Sending open queries!", ds.Conn.RemoteAddr())
	msgChan, errChan := b.DB.AsyncGetDirectMessages(ds.Ctx, b.upstreamConn.Nick, 99999999)

	recvCount := 0

	for chatMsg := range msgChan {
		recvCount++
		// Rewrite historical notices as private messages so clients
		// like HexChat are forced to open a query window for them.
		playbackCmd := chatMsg.Command
		if playbackCmd == "NOTICE" {
			playbackCmd = "PRIVMSG"
		}

		// Skip replying direct CTCP messages if they somehow ended up in the backlog.
		if strings.HasPrefix(chatMsg.Content, "\x01") {
			log.Debug().Msgf("[downstream %s] Skipping direct CTCP message from DB channel: %s", ds.Conn.RemoteAddr(), chatMsg.Content)
			continue
		}

		forMsg := ircmsg.MakeMessage(nil, chatMsg.Source, playbackCmd, ds.Nick, chatMsg.Content)

		// Add IRCv3 server-time tag if the client negotiated it during the CAP phase
		if ds.Caps["server-time"] {
			timeStr := time.Unix(chatMsg.Timestamp, 0).Format(time.RFC3339Nano)
			forMsg.SetTag("time", timeStr)
		}

		// Send to the downstream client
		if err := b.SendToClient(ds, forMsg); err != nil {
			log.Debug().Msgf("[downstream %s] Write failed during PM history: %v", ds.Conn.RemoteAddr(), err)
			// If the socket dies, remove the client. This fires ds.Cancel(),
			// which kills the SQLite query running in the DB worker.
			//b.DisconnectDownstreamConnection(ds, "Graceful")
			return // Kill this goroutine
		}
	}

	log.Debug().Msgf("[downstream %s] Message channel closed. Total received: %d. Waiting for error channel...", ds.Conn.RemoteAddr(), recvCount)

	if err := <-errChan; err != nil {
		log.Debug().Msgf("[downstream %s] Stream ended with error: %v", ds.Conn.RemoteAddr(), err)
	} else {
		log.Debug().Msgf("[downstream %s] SUCCESS! Stream ended cleanly with no errors.", ds.Conn.RemoteAddr())
	}
}

func (b *Bouncer) sendNamesList(ds *DownstreamConnection, chState *ChannelState) {
	// b.mu lock is needed if the caller does NOT clone the channels map.

	log.Debug().Msgf("[downstream %s] Sending /NAMES for %s", ds.Conn.RemoteAddr(), chState.Name)

	// 3. The Names List (353) - Chunked for Massive Channels

	// Pre-allocate the exact capacity needed to avoid costly slice resizing
	allNames := make([]string, 0, len(chState.Users))
	for nick, userPrefix := range chState.Users {
		allNames = append(allNames, userPrefix+nick)
	}

	// The safe IRC line limit is 512 bytes, although most servers can go over that (ex. Libera)
	// We use 400 as a safe threshold to leave plenty of room for the
	// ":server 353 nick = #channel :" boilerplate formatting.
	const maxPayloadLen = 400

	// Curren
	var currentChunk []string
	currentLen := 0

	// Start processing names
	for _, name := range allNames {
		// Flush chunk when it gets too big
		// +1 accounts for the space character used in strings.Join
		if currentLen+len(name)+1 > maxPayloadLen && len(currentChunk) > 0 {
			// Flush the current chunk to the client
			namesStr := strings.Join(currentChunk, " ")
			namesMsg := ircmsg.MakeMessage(nil, b.ServerName, "353", ds.Nick, "=", chState.Name, namesStr)
			b.SendToClient(ds, namesMsg)

			// Reset the chunker efficiently without reallocating memory
			currentChunk = currentChunk[:0]
			currentLen = 0
		}

		currentChunk = append(currentChunk, name)
		currentLen += len(name) + 1
	}

	// Flush any remaining names in the final chunk
	if len(currentChunk) > 0 {
		namesStr := strings.Join(currentChunk, " ")
		namesMsg := ircmsg.MakeMessage(nil, b.ServerName, "353", ds.Nick, "=", chState.Name, namesStr)
		b.SendToClient(ds, namesMsg)
	}

	// 4. End of Names (366)
	endMsg := ircmsg.MakeMessage(nil, b.ServerName, "366", ds.Nick, chState.Name, "End of /NAMES list.")
	b.SendToClient(ds, endMsg)
}
