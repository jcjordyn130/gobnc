package models

type UserState struct {
	// nick!user@host
	// nickname!username@host (realname)
	Nickname string
	Username string
	Host     string
	Realname string

	// Connected node
	Server string

	// WHO flags
	Away  bool
	IRCOp bool

	// Mapping of channel to prefix
	// Obtained from NAMES on individual channels
	ChanPrefixes map[string]string

	// Additional data
	AwayMessage string // WHOIS or away-notify
}
