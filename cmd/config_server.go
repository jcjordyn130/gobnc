package cmd

import (
	"bouncer/database"
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
)

// 1. Define the single type that holds dependencies for this subcommand domain
type ServerCmd struct {
	db *database.DB
	// You could add other specific dependencies here later, like:
	// logger *zap.Logger
}

// 2. Create a constructor to enforce dependency injection
func NewServerCmd(db *database.DB) *ServerCmd {
	return &ServerCmd{
		db: db,
	}
}

func (c *ServerCmd) newServer(ctx context.Context, cmd *cli.Command) error {
	// Grab username
	username := cmd.String("username")

	// Grab user
	user, err := c.db.GetUserByUsername(username)
	if err != nil {
		return err
	}

	// Add new server
	s := c.db.NewServer(*user)

	s.Domain = cmd.String("domain")
	s.Port = cmd.Int("port")
	s.Ssl = cmd.Bool("ssl")
	s.Name = cmd.String("name")

	// Commit to DB
	err = c.db.AddServer(s)
	if err != nil {
		return err
	}

	return nil
}

func (c *ServerCmd) listServers(ctx context.Context, cmd *cli.Command) error {
	// Grab servers
	servers, err := c.db.GetAllServers()
	if err != nil {
		return err
	}

	fmt.Println("------------------")
	fmt.Println("Servers:")
	fmt.Println("------------------")
	for _, server := range servers {
		log.Info().Msgf("%+v", server)
	}

	return nil
}

func (c *ServerCmd) Command() *cli.Command {
	return &cli.Command{
		Name:  "server",
		Usage: "server operations",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "username",
				Aliases:     []string{"u"},
				Usage:       "operate on servers for `USERNAME`",
				DefaultText: "",
				Required:    true,
			},
		},
		Commands: []*cli.Command{
			{
				Name:      "add",
				Usage:     "adds a new server",
				ArgsUsage: "",
				Action:    c.newServer,
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:        "domain",
						Aliases:     []string{"d"},
						Usage:       "domain to use",
						DefaultText: "",
						Required:    true,
					},
					&cli.StringFlag{
						Name:        "name",
						Aliases:     []string{"n"},
						Usage:       "name to use",
						DefaultText: "",
						Required:    true,
					},
					&cli.IntFlag{
						Name:        "port",
						Aliases:     []string{"p"},
						Usage:       "port to use",
						DefaultText: "",
						Required:    true,
					},
					&cli.BoolFlag{
						Name:        "ssl",
						Aliases:     []string{"s"},
						Usage:       "is the port SSL or plain-text",
						DefaultText: "",
						Required:    false,
						Value:       false,
					},
				},
			},
			{
				Name:      "list",
				Usage:     "lists all servers",
				ArgsUsage: "",
				Action:    c.listServers,
			},
		},
	}
}
