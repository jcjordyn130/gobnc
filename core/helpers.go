package core

import "github.com/rs/zerolog/log"

// Helper to separate prefixes from nicknames
func parsePrefix(rawNick string) (nick string, prefix string) {
	if len(rawNick) == 0 {
		return "", ""
	}
	// Common IRC prefixes
	switch rawNick[0] {
	case '~', '&', '@', '%', '+':
		return rawNick[1:], string(rawNick[0])
	default:
		return rawNick, ""
	}
}

// Helper to map IRC mode characters to NAMES list prefixes
func modeToPrefix(mode rune) string {
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
		log.Debug().Msgf("Invalid rune %c receieved for modeToPrefix", mode)
		return ""
	}
}
