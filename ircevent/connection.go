package ircevent

import (
	"fmt"
	"net"
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

	// Start thy loops
	go us.readLoop()
	go us.writeLoop()

	// Start handshake
	us.sendHandshake()

	// loop forever
	// TODO: actually listen to the context
	for {
	}
}
