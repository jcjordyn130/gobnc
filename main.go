package main

import (
	"crypto/tls"
	"fmt"
	"log"

	"bouncer/config"
	"bouncer/core"
	"bouncer/database"
	"bouncer/downstreamHandlers"
	handlers "bouncer/downstreamHandlers"

	_ "net/http/pprof" // The blank identifier is required here to register the handlers

	"github.com/ergochat/irc-go/ircevent"
)

func main() {
	conf, err := config.LoadConfig()
	if err != nil {
		panic(err)
	}

	conn := ircevent.Connection{
		Server: fmt.Sprintf("%s:%d", conf.UpstreamServer, conf.UpstreamPort),
		UseTLS: conf.UseTLS,
		Nick:   conf.Nick,
		Debug:  conf.Verbose,
	}

	if conf.IgnoreCerts {
		log.Println("[main] Ignoring TLS errors due to config...")
		conn.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	}

	if conf.UpstreamPassword != "" {
		conn.Password = conf.UpstreamPassword
	}

	// Init bouncer
	b := core.NewBouncer(&conn)

	// Init database
	bdb, err := database.NewDB(conf.DBPath, conf.MaxQLen)
	if err != nil {
		panic(err)
	}
	b.DB = *bdb

	// Register downstream handlers
	// These are commands sent by clients connected to us
	b.Register("PING", handlers.HandlePING)
	b.Register("PRIVMSG", handlers.HandlePRIVMSG)
	b.Register("WHO", handlers.HandleWHO)
	b.Register("USERHOST", handlers.HandleUSERHOST)
	b.Register("NICK", handlers.HandleNICK)
	b.Register("JOIN", handlers.HandleJOIN)
	b.Register("QUIT", downstreamHandlers.HandleQUIT)

	// Connect to upstream server
	err = b.ConnectToServer(&conn)
	if err != nil {
		panic("failed to connect to server")
	}

	// Start upstream server loop
	log.Println("Starting upstream server loop")
	go conn.Loop()

	// Start downstream listener
	b.ListenDownstream("127.0.0.1:12345")
}
