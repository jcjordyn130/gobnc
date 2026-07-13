// core/handshake.go
// This file only contains the function for an IRC handshake (USER, NICK, and cap negotation)
//
// I split this off into its own file because of how long the function is
// TODO: don't use raw IRC messages
package core

import (
	"bouncer/models"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/ergochat/irc-go/ircmsg"
	"github.com/ergochat/irc-go/ircreader"
	"github.com/rs/zerolog/log"
)

// This is a basic reader loop for a handshake as it was easier to do this
// than to implement handshake handling in the main loop.
func (b *Bouncer) newHandshake(reader ircreader.Reader, ds *DownstreamConnection) error {
	log.Debug().Msgf("[downstream %s] Starting handshake!", ds.Conn.RemoteAddr())

	for {
		// read raw line
		rawLine, err := reader.ReadLine()
		if err != nil {
			log.Error().Msgf("[downstream %s] handshake error: %v", ds.Conn.RemoteAddr(), err)
			return err
		}

		// Parse it
		line, err := ircmsg.ParseLine(string(rawLine))
		if err != nil {
			log.Error().Msgf("[downstream %s] handshake error: %v", ds.Conn.RemoteAddr(), err)
			return err
		}

		// Handle it
		err = b.handleHandshakeLine(line, ds)
		if err != nil {
			log.Error().Msgf("[downstream %s] handshake error: %v", ds.Conn.RemoteAddr(), err)
			return err
		}

		// Return if we're done
		if !ds.HandshakeInProgress && ds.HandshakeComplete {
			err := b.sendWelcome(ds)
			if err != nil {
				return err
			}

			log.Debug().Msgf("[downstream %s] handshake complete, returning to main client loop", ds.Conn.RemoteAddr())
			return nil
		}
	}
}

func (b *Bouncer) sendWelcome(ds *DownstreamConnection) error {
	log.Debug().Msgf("[downstream %s] Sending welcome burst (RPL_001, MOTD, and backlog)", ds.Conn.RemoteAddr())

	// Send RPL_001 WELCOME
	if b.upstreamConn.Connected() {
		log.Debug().Msgf("[downstream %s] sending upstream nick %s to client", ds.Conn.RemoteAddr(), b.GetUpstreamConn().CurrentNick())
		rplWelcome := ircmsg.MakeMessage(nil, b.ServerName, "001", b.GetUpstreamConn().CurrentNick(), "Welcome to the Golang BNC!")
		ds.Nick = b.GetUpstreamConn().CurrentNick()
		ds.SendToClient(rplWelcome)
	} else {
		log.Debug().Msgf("[downstream %s] sending client nick %s to client due to no connection!", ds.Conn.RemoteAddr(), ds.Nick)
		rplWelcome := ircmsg.MakeMessage(nil, b.ServerName, "001", ds.Nick, "Welcome to the Golang BNC!")
		ds.SendToClient(rplWelcome)
	}

	// Send MOTD
	b.SendCachedMOTD(ds)

	// Send channel joins
	go b.sendJoinedChannels(ds)

	// Send open queries
	go b.sendOpenQueries(ds)

	return nil
}

func (b *Bouncer) handleHandshakeLine(line ircmsg.Message, ds *DownstreamConnection) error {
	log.Debug().Msgf("[downstream %s] Handling line from handshake: %+v", ds.Conn.RemoteAddr(), line)

	switch line.Command {
	case "CAP":
		err := b.handleHandshakeCAP(line, ds)
		if err != nil {
			return err
		}
	case "NICK":
		// This denotes start of handshake UNLESS caps are in use, which if they are then
		// CAP *should* be the first command sent so this would be true anyways.
		ds.HandshakeInProgress = true

		if len(line.Params) > 0 {
			log.Debug().Msgf("[downstream %s] setting nickname to %s", ds.Conn.RemoteAddr(), line.Params[0])
			ds.Nick = line.Params[0]
		}

	case "USER":
		if len(line.Params) >= 4 {
			// TODO: maybe we should store the username and realname somewhere? idk
			log.Debug().Msgf("[downstream %s] received USER command %s", ds.Conn.RemoteAddr(), line.Params[0])

			// This *officially* denotes end of handshake IF caps aren't used
			// This variable is only set IF CAP {LS, REQ} is sent
			if !ds.HandshakeInProgress {
				ds.HandshakeInProgress = false
				ds.HandshakeComplete = true
			}
		}

	default:
		// To free up precious OS resources, assume any line that isn't handled above is malicious.
		log.Warn().Msgf("[downstream %s] Invalid message received during handshake, DISCONNECTING", ds.Conn.RemoteAddr())
		return fmt.Errorf("invalid handshake")
	}

	return nil
}

func (b *Bouncer) handleHandshakeCAP(line ircmsg.Message, ds *DownstreamConnection) error {
	log.Debug().Msgf("[downstream %s] Handling CAP command", ds.Conn.RemoteAddr())

	// Technically an error but not worth one passing back up the stack and killing the handshake.
	if len(line.Params) < 1 {
		log.Warn().Msgf("[downstream %s] CAP params < 1, invalid!", ds.Conn.RemoteAddr())
		return nil
	}

	subCmd := line.Params[0]
	if subCmd == "LS" {
		// Client is starting capability negotiation
		ds.HandshakeInProgress = true

		// Get supported version (if any)
		// TODO: actually use this and implement support for version extentions
		if len(line.Params) == 2 {
			ds.CapVersionSupported = line.Params[1]
		}

		log.Debug().Msgf("[downstream %s] client is asking for cap list", ds.Conn.RemoteAddr())

		if ds.CapVersionSupported != "" {
			log.Debug().Msgf("[downstream %s] client supports CAP version %s", ds.Conn.RemoteAddr(), ds.CapVersionSupported)
		}

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
		ds.SendToClient(capMsg)
	} else if subCmd == "REQ" {
		// Client is starting capability negotiation
		ds.HandshakeInProgress = true

		requestedCaps := line.Params[1]

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
			ds.SendToClient(capMsg)
		}

		// Batch send the NAKs (declined capabilities)
		if len(nakedCaps) > 0 {
			nakStr := strings.Join(nakedCaps, " ")
			capMsg := ircmsg.MakeMessage(nil, b.ServerName, "CAP", "*", "NAK", nakStr)
			ds.SendToClient(capMsg)
		}
	} else if subCmd == "END" {
		log.Debug().Msgf("[downstream %s] cap negotiation end", ds.Conn.RemoteAddr())

		// If CAP {LS, REQ} is sent, then handshake does NOT end until CAP END is sent.
		ds.HandshakeInProgress = false
		ds.HandshakeComplete = true
	}

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
		ds.SendToClient(joinMsg)

		// 2. The Topic (332)
		if chanState.Topic != "" {
			ds.SendToClient(ircmsg.MakeMessage(nil, b.ServerName, "332", ds.Nick, chanState.Name, chanState.Topic))
		}

		// 3. Channel Modes (324)
		if chanState.Modes != "" {
			ds.SendToClient(ircmsg.MakeMessage(nil, b.ServerName, "324", ds.Nick, chanState.Name, chanState.Modes))
		}

		// 4. Creation Time (329)
		if chanState.CreationTime != "" {
			ds.SendToClient(ircmsg.MakeMessage(nil, b.ServerName, "329", ds.Nick, chanState.Name, chanState.CreationTime))
		}

		// Send /NAMES list
		b.sendNamesList(ds, chanState)

		// Send history
		go b.SendHistory(&chanState.Name, ds)
	}
}

func (b *Bouncer) sendOpenQueries(ds *DownstreamConnection) {
	log.Debug().Msgf("[downstream %s] Sending open queries!", ds.Conn.RemoteAddr())
	msgChan, errChan := b.DB.AsyncGetDirectMessages(ds.Ctx, b.upstreamConn.CurrentNick(), 99999999)

	recvCount := 0

	// Create single message structure to reuse
	// These allocate a TON of memory when being sent this quickly
	reuseMsg := ircmsg.MakeMessage(nil, "", "PRIVMSG", ds.Nick, "")

	for chatMsg := range msgChan {
		recvCount++

		// Skip replying direct CTCP messages if they somehow ended up in the backlog.
		if strings.HasPrefix(chatMsg.Content, "\x01") {
			log.Debug().Msgf("[downstream %s] Skipping direct CTCP message from DB channel: %s", ds.Conn.RemoteAddr(), chatMsg.Content)
			continue
		}

		// Rewrite historical notices as private messages so clients
		// like HexChat are forced to open a query window for them.
		//
		// ONLY do this for user (not server) messages, otherwise you get a DM
		// from irc.libera.chat
		playbackCmd := chatMsg.Command
		if playbackCmd == "NOTICE" && strings.Contains(chatMsg.Source, "!") {
			playbackCmd = "PRIVMSG"
		}

		// Modify the Message structure directly
		reuseMsg.Source = chatMsg.Source
		reuseMsg.Command = playbackCmd

		// Overwrite existing parms
		reuseMsg.Params[0] = ds.Nick
		reuseMsg.Params[1] = chatMsg.Content

		// Add IRCv3 server-time tag if the client negotiated it during the CAP phase
		if ds.Caps["server-time"] {
			timeStr := time.Unix(chatMsg.Timestamp, 0).Format(time.RFC3339Nano)
			reuseMsg.SetTag("time", timeStr)
		}

		// Send to the downstream client
		if err := ds.SendToClient(reuseMsg); err != nil {
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

func (b *Bouncer) sendNamesList(ds *DownstreamConnection, chState *models.ChannelState) {
	log.Debug().Msgf("[downstream %s] Sending /NAMES for %s", ds.Conn.RemoteAddr(), chState.Name)

	// 1. Gather all users for this specific channel from the global state
	var allNames []string

	b.user_mu.RLock()
	for nick, userState := range b.Users {
		// Check if the user's global state shows they are in this channel
		if prefix, inChan := userState.ChanPrefixes[chState.Name]; inChan {
			allNames = append(allNames, prefix+nick)
		}
	}
	b.user_mu.RUnlock()

	// The safe IRC line limit is 512 bytes, although most servers can go over that (ex. Libera)
	// We use 400 as a safe threshold to leave plenty of room for the
	// ":server 353 nick = #channel :" boilerplate formatting.
	const maxPayloadLen = 400

	// Current chunk tracking
	var currentChunk []string
	currentLen := 0

	// 2. Start processing names into safe 400-byte chunks
	for _, name := range allNames {
		// Flush chunk when it gets too big
		// +1 accounts for the space character used in strings.Join
		if currentLen+len(name)+1 > maxPayloadLen && len(currentChunk) > 0 {
			// Flush the current chunk to the client
			namesStr := strings.Join(currentChunk, " ")
			namesMsg := ircmsg.MakeMessage(nil, b.ServerName, "353", ds.Nick, "=", chState.Name, namesStr)
			ds.SendToClient(namesMsg)

			// Reset the chunker efficiently without reallocating memory
			currentChunk = currentChunk[:0]
			currentLen = 0
		}

		currentChunk = append(currentChunk, name)
		currentLen += len(name) + 1
	}

	// 3. Flush any remaining names in the final chunk
	if len(currentChunk) > 0 {
		namesStr := strings.Join(currentChunk, " ")
		namesMsg := ircmsg.MakeMessage(nil, b.ServerName, "353", ds.Nick, "=", chState.Name, namesStr)
		ds.SendToClient(namesMsg)
	}

	// 4. End of Names (366)
	endMsg := ircmsg.MakeMessage(nil, b.ServerName, "366", ds.Nick, chState.Name, "End of /NAMES list.")
	ds.SendToClient(endMsg)
}
