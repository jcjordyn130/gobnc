package cmd

import (
	"bouncer/database"
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
)

// 1. Define the single type that holds dependencies for this subcommand domain
type UserCmd struct {
	db *database.DB
	// You could add other specific dependencies here later, like:
	// logger *zap.Logger
}

// 2. Create a constructor to enforce dependency injection
func NewUserCmd(db *database.DB) *UserCmd {
	return &UserCmd{
		db: db,
	}
}

func (c *UserCmd) addUser(ctx context.Context, cmd *cli.Command) error {
	name := cmd.Args().First()
	if name == "" {
		return fmt.Errorf("error: user name is required")
	}

	password := cmd.Args().Get(1)
	if password == "" {
		return fmt.Errorf("error: password is required")
	}

	// Create new user struct
	user := c.db.NewUser(name)
	user.SetPassword(password)

	// Add to DB
	err := c.db.AddUser(user)
	if err != nil {
		return err
	}

	return nil
}

func (c *UserCmd) removeUser(ctx context.Context, cmd *cli.Command) error {
	name := cmd.Args().First()
	if name == "" {
		return fmt.Errorf("error: user name is required")
	}

	err := c.db.RemoveUser(name)
	if err != nil {
		return err
	}

	return nil
}

func (c *UserCmd) validateUser(ctx context.Context, cmd *cli.Command) error {
	// Get arguments
	username := cmd.Args().First()
	if username == "" {
		return fmt.Errorf("error: username is required")
	}

	password := cmd.Args().Get(1)
	if password == "" {
		return fmt.Errorf("error: password is required")
	}

	// Get datatypes and verify
	user, err := c.db.GetUserByUsername(username)
	if err != nil {
		return err
	}

	isValid, err := user.VerifyPassword(password)
	if err != nil {
		return err
	}

	if isValid {
		log.Info().Msgf("Password for user %s is VALID", username)
	} else {
		log.Info().Msgf("Password for user %s is NOT VALID", username)
	}

	return nil
}

func (c *UserCmd) changePW(ctx context.Context, cmd *cli.Command) error {
	// Get arguments
	username := cmd.Args().First()
	if username == "" {
		return fmt.Errorf("error: username is required")
	}

	password := cmd.Args().Get(1)
	if password == "" {
		return fmt.Errorf("error: new password is required")
	}

	// Get datatypes and verify
	user, err := c.db.GetUserByUsername(username)
	if err != nil {
		return err
	}

	// Update password
	user.SetPassword(password)

	// Commit user
	err = c.db.UpdateUser(*user)
	if err != nil {
		return err
	}

	return nil
}

func (c *UserCmd) listUsers(ctx context.Context, cmd *cli.Command) error {
	// Get users
	users, err := c.db.GetAllUsers()
	if err != nil {
		return err
	}

	fmt.Println("------------------")
	fmt.Println("Users:")
	fmt.Println("------------------")
	for _, user := range users {
		fmt.Printf("%s\n", user.Username)
	}

	return nil
}

func (c *UserCmd) Command() *cli.Command {
	return &cli.Command{
		Name:  "user",
		Usage: "configure the userstore",
		Commands: []*cli.Command{
			{
				Name:      "add",
				Usage:     "add a user to the userstore",
				ArgsUsage: "<username> <password>",
				Action:    c.addUser,
			},
			{
				Name:      "remove",
				Aliases:   []string{"rm", "del"},
				Usage:     "remove a user from the userstore",
				ArgsUsage: "<username>",
				Action:    c.removeUser,
			},
			{
				Name:    "list",
				Aliases: []string{"ls"},
				Usage:   "list all autojoin channels",
				Action:  c.listUsers,
			},
			{
				Name:      "validate",
				Usage:     "validates a password against a username",
				ArgsUsage: "<username> <password>",
				Action:    c.validateUser,
			},
			{
				Name:  "update",
				Usage: "update a user",
				Commands: []*cli.Command{
					{
						Name:      "password",
						Usage:     "changes a users password",
						ArgsUsage: "<username> <password>",
						Action:    c.changePW,
					},
				},
			},
		},
	}
}
