package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	natsgo "github.com/nats-io/nats.go"
	"github.com/rs/zerolog"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/config"
	"github.com/lidisoft/horizon/internal/configdef"
	"github.com/lidisoft/horizon/internal/identity"
	"github.com/lidisoft/horizon/internal/issue"
	"github.com/lidisoft/horizon/internal/logging"
	"github.com/lidisoft/horizon/internal/migrations"
	platformmail "github.com/lidisoft/horizon/internal/platform/mail"
	"github.com/lidisoft/horizon/internal/platform/metrics"
	"github.com/lidisoft/horizon/internal/platform/nats"
	"github.com/lidisoft/horizon/internal/platform/postgres"
	"github.com/lidisoft/horizon/internal/platform/s3"
	"github.com/lidisoft/horizon/internal/project"
	"github.com/lidisoft/horizon/internal/server"
	"github.com/lidisoft/horizon/internal/version"
	"github.com/lidisoft/horizon/internal/workspace"
)

// runtime holds the connections shared by serve and worker.
type runtime struct {
	cfg     config.Config
	log     zerolog.Logger
	pool    *pgxpool.Pool
	nc      *natsgo.Conn
	s3      *s3.Client
	metrics *metrics.Registry
}

// open loads the configuration and connects to every dependency. Any failure
// aborts startup: the process must not run half-configured.
func open(ctx context.Context, need config.Need, system string) (*runtime, error) {
	cfg, err := config.FromEnv(need)
	if err != nil {
		return nil, err
	}
	log := logging.New(cfg.LogLevel, cfg.LogFormat, system)
	log.Info().Str("version", version.String()).Msg("starting")

	rt := &runtime{cfg: cfg, log: log, metrics: metrics.New()}
	if rt.pool, err = postgres.Connect(ctx, cfg.DatabaseURL, logging.WithSystem(log, "postgres")); err != nil {
		return nil, err
	}
	if rt.nc, err = nats.Connect(cfg.NatsURL, "horizon-"+system, logging.WithSystem(log, "nats")); err != nil {
		rt.close()
		return nil, err
	}
	if rt.s3, err = s3.New(cfg.S3); err != nil {
		rt.close()
		return nil, err
	}
	return rt, nil
}

func (rt *runtime) close() {
	if rt.nc != nil {
		rt.nc.Close()
	}
	if rt.pool != nil {
		rt.pool.Close()
	}
}

func (rt *runtime) mailer() platformmail.Sender {
	if rt.cfg.SMTPURL == nil {
		return nil
	}
	return platformmail.NewSMTP(rt.cfg.SMTPURL, rt.cfg.SMTPFrom)
}

// identity returns the identity service.
func (rt *runtime) identity() *identity.Service {
	return identity.New(identity.Options{
		Pool:          rt.pool,
		Log:           logging.WithSystem(rt.log, "identity"),
		Mail:          rt.mailer(),
		BaseURL:       rt.cfg.BaseURL,
		SessionSecret: rt.cfg.SessionSecret,
	})
}

// api registers every module's routes on /api/v1.
func (rt *runtime) api() func(api *gin.RouterGroup) {
	ids := rt.identity()
	auth := identity.NewHTTP(ids, rt.cfg.BaseURL.Scheme == "https")
	acc := access.New(rt.pool, nil)
	config := configdef.New(rt.pool, acc, nil, nil)
	workspaces := workspace.NewHTTP(workspace.New(workspace.Options{
		Pool:     rt.pool,
		Log:      logging.WithSystem(rt.log, "workspace"),
		Identity: ids,
		Access:   acc,
		Mail:     rt.mailer(),
		BaseURL:  rt.cfg.BaseURL,
		OnCreate: []workspace.CreateHook{config.SeedDefaults},
	}), auth)
	roles := access.NewHTTP(acc)
	projectSvc := project.New(rt.pool, acc, nil)
	projects := project.NewHTTP(projectSvc)
	configHTTP := configdef.NewHTTP(config)
	projectSvc.OnDelete(config.DeleteProject)
	issues := issue.NewHTTP(issue.New(rt.pool, acc, config, projectSvc, nil), projectSvc)
	return func(api *gin.RouterGroup) {
		api.Use(auth.Authenticate())
		auth.Register(api)
		workspaces.Register(api, roles.Register, configHTTP.RegisterWorkspace, issues.RegisterWorkspace, func(ws *gin.RouterGroup) {
			projects.Register(ws, configHTTP.RegisterProject, issues.RegisterProject)
		})
	}
}

// checks are the readiness probes (docs/operations.md#health).
func (rt *runtime) checks() []server.Check {
	return []server.Check{
		{Name: "postgres", Check: func(ctx context.Context) error { return rt.pool.Ping(ctx) }},
		{Name: "nats", Check: func(context.Context) error {
			if !rt.nc.IsConnected() {
				return errors.New("not connected")
			}
			return nil
		}},
		{Name: "s3", Check: rt.s3.Ping},
		{Name: "schema", Check: rt.checkSchema},
	}
}

// checkSchema fails when the database is older than this binary expects, not
// when it is newer (expand/contract upgrades run migrations first).
func (rt *runtime) checkSchema(ctx context.Context) error {
	want, err := migrations.Expected()
	if err != nil {
		return err
	}
	db := stdlib.OpenDBFromPool(rt.pool)
	defer db.Close()
	got, err := migrations.Current(ctx, db)
	if err != nil {
		return err
	}
	if got < want {
		return fmt.Errorf("schema at version %d, need %d: run horizon migrate up", got, want)
	}
	return nil
}
