package ircevent

import (
	"errors"
	"os"
	"strings"
	"time"

	"github.com/ergochat/irc-go/ircmsg"
)

// List of IRCv3 capabilities that we support
var SupportedCaps = map[string]bool{
	"server-time":  true,
	"echo-message": true,
	"multi-prefix": true,
}

func (us *UpstreamConnection) parseMsg(rawmsg []byte) (*ircmsg.Message, error) {
	line := string(rawmsg)
	msg, err := ircmsg.ParseLine(line)
	if err != nil {
		us.logger.Debug().Msgf("Error parsing line")
		return nil, err
	}

	return &msg, nil
}

func (us *UpstreamConnection) sendHandshake() error {
	us.conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	// Ask for CAP list
	us.WriteMsgNoQ(ircmsg.MakeMessage(nil, "", "CAP LS"))

	// Wait for caps list
	resp, err := us.reader.ReadLine()
	if err != nil {
		if errors.Is(err, os.ErrDeadlineExceeded) {
			us.logger.Warn().Msgf("Server did not respond to CAP withint 5 seconds... proceeding")
		}

		us.logger.Error().Msgf("error occurred during CAP: %v", err)
		return err
	}

	parsedResp, err := us.parseMsg(resp)
	if err != nil {
		return err
	}

	us.handleCAP(*parsedResp)

	return nil
}

func (us *UpstreamConnection) handleCAP(msg ircmsg.Message) {
	if len(msg.Params) < 2 {
		us.logger.Warn().Msgf("Invalid CAP: %+v", msg)
		return
	}

	switch msg.Params[1] {
	case "LS":
		requestedCaps := make([]string, 0)

		// Build supported CAP list
		for cap := range strings.SplitSeq(msg.Params[2], " ") {
			us.logger.Info().Msgf("Server supports cap '%s'", cap)
			if SupportedCaps[cap] == true {
				us.logger.Info().Msgf("We support cap '%s', adding to list", cap)
				requestedCaps = append(requestedCaps, cap)
			}
		}

		// Request CAPs
		us.WriteMsg(ircmsg.MakeMessage(nil, "", "CAP", "REQ", strings.Join(requestedCaps, " ")))

		// Early return
		return
	case "ACK":
		for cap := range strings.SplitSeq(msg.Params[2], " ") {
			us.logger.Info().Msgf("Server ACKed cap '%s'", cap)
			us.CurrentCaps = append(us.CurrentCaps, cap)
		}
	case "NAK":
		us.logger.Warn().Msgf("Server responded with NAK to supposedly supported CAPs!")
		for cap := range strings.SplitSeq(msg.Params[2], " ") {
			us.logger.Warn().Msgf("Server NAKed cap '%s'", cap)
		}
	}

	// End capability neg
	us.WriteMsg(ircmsg.MakeMessage(nil, "", "CAP", "END"))
}
