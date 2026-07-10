package core

import (
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
)

// @ +o Operator
// ~ +q Founder
// & +a Admin
// % +h Half-Operator
// + +v Voice
var validNickPrefixes string = "~&@%+"

func parsePrefix(rawNick string) (nick string, prefix string) {
	// Avoid crashing below if we are, somehow, given a blank nick
	if len(rawNick) == 0 {
		log.Debug().Msg("Blank raw nickname given to parsePrefix")
		return "", ""
	}

	var prefixes strings.Builder
	var nickStart int

	for i, char := range rawNick {
		if strings.ContainsRune(validNickPrefixes, char) {
			prefixes.WriteString(string(char))
			nickStart = i + 1
		} else {
			break
		}
	}

	log.Debug().Str("nick", rawNick[nickStart:]).Str("prefix", prefixes.String()).Msgf("Parsed prefix/nick for raw nick %s", rawNick)
	return rawNick[nickStart:], prefixes.String()
}

// Helper to map IRC mode characters to NAMES list prefixes
func modeToPrefix(mode rune) string {
	log.Trace().Msgf("Parsing mode %c to prefix", mode)
	switch mode {
	case 'o':
		return "@" // Operator
	case 'v':
		return "+" // Voice
	case 'h':
		return "%" // Half-Op
	case 'a':
		return "&" // Admin
	case 'q':
		return "~" // Founder
	default:
		BUG(fmt.Sprintf("Invalid rune %c receieved for modeToPrefix", mode))
		return ""
	}
}

func BUG(errString string) {
	log.Warn().Msgf("[BUG]: %s", errString)
	log.Warn().Msg("Please run at a debug loglevel and make a bug report. Thank you!")
}
