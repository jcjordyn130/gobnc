package ircevent

import (
	"bufio"
	"fmt"
	"net"

	"github.com/ergochat/irc-go/ircreader"
)

func (us *UpstreamConnection) Connect() error {
	// Connect to server
	serv := fmt.Sprintf("%s:%s", us.Config.Server, us.Config.Port)
	conn, err := net.Dial("tcp", serv)
	if err != nil {
		return err
	}
	defer conn.Close()

	us.conn = conn
	us.reader = ircreader.NewIRCReader(us.conn)
	us.writer = bufio.NewWriter(us.conn)

	// Start handshake
	err = us.sendHandshake()
	if err != nil {
		us.connected = false
		return err
	}

	// Start thy loops
	go us.readLoop()
	go us.writeLoop()

	// loop forever
	// TODO: actually listen to the context
	for {
	}
}

func (us *UpstreamConnection) Connected() bool {
	return us.connected
}

func (us *UpstreamConnection) CurrentNick() string {
	return "%%TESTING%%"
}

func (us *UpstreamConnection) SetNick(nick string) {
	us.logger.Trace().Msgf("Setting nick to %s", nick)
}

func (us *UpstreamConnection) Join(channel string) {
	us.logger.Trace().Msgf("Joining channel %s", channel)
}
