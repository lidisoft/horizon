package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/urfave/cli/v3"

	"github.com/lidisoft/horizon/internal/apperr"
	"github.com/lidisoft/horizon/internal/config"
	"github.com/lidisoft/horizon/internal/identity"
	"github.com/lidisoft/horizon/internal/logging"
	"github.com/lidisoft/horizon/internal/platform/postgres"
)

// adminCommand groups instance administration (docs/operations.md#admin-cli).
// Workspaces arrive with their module; reconcile, rekey and rebuild-reports
// later.
func adminCommand() *cli.Command {
	email := &cli.StringFlag{Name: "email", Usage: "the account's email", Required: true}
	return &cli.Command{
		Name:  "admin",
		Usage: "Administer the instance",
		Commands: []*cli.Command{
			{
				Name:  "create-user",
				Usage: "Create an account; prints a link to choose the password unless --password-stdin",
				Flags: []cli.Flag{
					email,
					&cli.StringFlag{Name: "name", Usage: "display name (default: the email's local part)"},
					&cli.BoolFlag{Name: "instance-admin", Usage: "grant instance administration"},
					&cli.BoolFlag{Name: "password-stdin", Usage: "read the password from the first line of standard input"},
				},
				Action: admin(createUser),
			},
			{
				Name:   "reset-password",
				Usage:  "Print a single-use link to choose a new password (valid one hour)",
				Flags:  []cli.Flag{email},
				Action: admin(resetPassword),
			},
			{
				Name:   "deactivate-user",
				Usage:  "Block sign-in instance-wide and revoke every session",
				Flags:  []cli.Flag{email},
				Action: admin(setDeactivated(true)),
			},
			{
				Name:   "reactivate-user",
				Usage:  "Allow a deactivated account to sign in again",
				Flags:  []cli.Flag{email},
				Action: admin(setDeactivated(false)),
			},
		},
	}
}

type adminFunc func(ctx context.Context, cmd *cli.Command, ids *identity.Service) error

// admin connects to PostgreSQL only: admin commands must work while NATS or
// S3 are down. BASE_URL is read when set, for links.
func admin(fn adminFunc) cli.ActionFunc {
	return func(ctx context.Context, cmd *cli.Command) error {
		cfg, err := config.FromEnv(config.NeedAdmin)
		if err != nil {
			return err
		}
		log := logging.New(cfg.LogLevel, cfg.LogFormat, "admin")
		pool, err := postgres.Connect(ctx, cfg.DatabaseURL, logging.WithSystem(log, "postgres"))
		if err != nil {
			return err
		}
		defer pool.Close()
		ids := identity.New(identity.Options{Pool: pool, Log: log, BaseURL: cfg.BaseURL, SessionSecret: cfg.SessionSecret})
		return describe(fn(ctx, cmd, ids))
	}
}

// describe turns validation errors into readable CLI errors.
func describe(err error) error {
	e, ok := apperr.As(err)
	if !ok {
		return err
	}
	if len(e.Fields) == 0 {
		return errors.New(strings.ReplaceAll(e.Code, "_", " "))
	}
	parts := make([]string, len(e.Fields))
	for i, f := range e.Fields {
		parts[i] = f.Field + ": " + strings.ReplaceAll(f.Code, "_", " ")
	}
	return errors.New(strings.Join(parts, "; "))
}

func createUser(ctx context.Context, cmd *cli.Command, ids *identity.Service) error {
	email := cmd.String("email")
	name := cmd.String("name")
	if name == "" {
		name, _, _ = strings.Cut(email, "@")
	}
	var password string
	if cmd.Bool("password-stdin") {
		var err error
		if password, err = readLine(os.Stdin); err != nil {
			return fmt.Errorf("read password: %w", err)
		}
		if password == "" {
			return errors.New("empty password on standard input")
		}
	}
	u, err := ids.CreateUser(ctx, uuid.NullUUID{}, identity.NewUser{
		Email: email, Name: name, Password: password, InstanceAdmin: cmd.Bool("instance-admin"),
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.Writer, "created %s (%s)\n", u.Email, u.ID)
	if password != "" {
		return nil
	}
	link, err := ids.IssuePasswordReset(ctx, u.ID)
	if err != nil {
		return fmt.Errorf("the account has no password yet: %w", err)
	}
	fmt.Fprintf(cmd.Writer, "choose a password within an hour: %s\n", link)
	return nil
}

func resetPassword(ctx context.Context, cmd *cli.Command, ids *identity.Service) error {
	u, err := ids.UserByEmail(ctx, cmd.String("email"))
	if err != nil {
		return err
	}
	link, err := ids.IssuePasswordReset(ctx, u.ID)
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.Writer, "reset link for %s, valid one hour: %s\n", u.Email, link)
	return nil
}

func setDeactivated(deactivated bool) adminFunc {
	return func(ctx context.Context, cmd *cli.Command, ids *identity.Service) error {
		u, err := ids.UserByEmail(ctx, cmd.String("email"))
		if err != nil {
			return err
		}
		if _, err := ids.SetDeactivated(ctx, uuid.NullUUID{}, u.ID, deactivated); err != nil {
			return err
		}
		state := "reactivated"
		if deactivated {
			state = "deactivated"
		}
		fmt.Fprintf(cmd.Writer, "%s %s\n", state, u.Email)
		return nil
	}
}

func readLine(r io.Reader) (string, error) {
	line, err := bufio.NewReader(r).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}
