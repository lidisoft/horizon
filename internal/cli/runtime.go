package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	natsgo "github.com/nats-io/nats.go"
	"github.com/rs/zerolog"

	"github.com/lidisoft/horizon/internal/access"
	"github.com/lidisoft/horizon/internal/config"
	"github.com/lidisoft/horizon/internal/configdef"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/identity"
	"github.com/lidisoft/horizon/internal/issue"
	"github.com/lidisoft/horizon/internal/logging"
	"github.com/lidisoft/horizon/internal/migrations"
	"github.com/lidisoft/horizon/internal/notify"
	"github.com/lidisoft/horizon/internal/planning"
	"github.com/lidisoft/horizon/internal/platform/egress"
	platformmail "github.com/lidisoft/horizon/internal/platform/mail"
	"github.com/lidisoft/horizon/internal/platform/metrics"
	"github.com/lidisoft/horizon/internal/platform/nats"
	"github.com/lidisoft/horizon/internal/platform/postgres"
	"github.com/lidisoft/horizon/internal/platform/s3"
	"github.com/lidisoft/horizon/internal/platform/secrets"
	"github.com/lidisoft/horizon/internal/project"
	"github.com/lidisoft/horizon/internal/realtime"
	"github.com/lidisoft/horizon/internal/report"
	"github.com/lidisoft/horizon/internal/search"
	"github.com/lidisoft/horizon/internal/server"
	"github.com/lidisoft/horizon/internal/sso"
	"github.com/lidisoft/horizon/internal/vcs"
	"github.com/lidisoft/horizon/internal/version"
	"github.com/lidisoft/horizon/internal/view"
	"github.com/lidisoft/horizon/internal/webhook"
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

// secrets is the box sealing secrets at rest; nil without SECRET_KEY
// (commands that don't read secrets).
func (rt *runtime) secrets() *secrets.Box {
	if rt.cfg.SecretKey == nil {
		return nil
	}
	box, err := secrets.New(rt.cfg.SecretKey, rt.cfg.SecretKeysPrevious...)
	if err != nil {
		panic(err) // the configuration checked the key lengths
	}
	return box
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

// domain holds the domain services, wired to each other.
type domain struct {
	identity   *identity.Service
	access     *access.Service
	config     *configdef.Service
	workspaces *workspace.Service
	projects   *project.Service
	issues     *issue.Service
	planning   *planning.Service
	notify     *notify.Service
	webhooks   *webhook.Service
	vcs        *vcs.Service
	sso        *sso.Service
}

// domain builds the domain services: serve and worker share the same graph,
// so policies run commands exactly as requests do.
func (rt *runtime) domain() domain {
	d := domain{identity: rt.identity()}
	d.access = access.New(rt.pool, nil)
	d.config = configdef.New(rt.pool, d.access, nil, nil)
	d.workspaces = workspace.New(workspace.Options{
		Pool:     rt.pool,
		Log:      logging.WithSystem(rt.log, "workspace"),
		Identity: d.identity,
		Access:   d.access,
		Mail:     rt.mailer(),
		BaseURL:  rt.cfg.BaseURL,
		OnCreate: []workspace.CreateHook{d.config.SeedDefaults},
	})
	d.projects = project.New(rt.pool, d.access, nil)
	d.projects.OnDelete(d.config.DeleteProject)
	d.issues = issue.New(rt.pool, d.access, d.config, d.projects, nil)
	d.issues.SetStorage(rt.s3)
	d.planning = planning.New(rt.pool, d.access, d.config, d.projects, d.issues, nil)
	prefix := eventlog.DefaultStream(rt.cfg.EventsRetention).Prefix
	d.notify = notify.New(notify.Options{
		Pool: rt.pool, Log: logging.WithSystem(rt.log, "notify"), Identity: d.identity, Access: d.access, Workspaces: d.workspaces, Issues: d.issues,
		Mail: rt.mailer(), BaseURL: rt.cfg.BaseURL,
		Publish: func(userID uuid.UUID, data []byte) {
			if err := rt.nc.Publish(realtime.InboxSubject(prefix, userID), data); err != nil {
				rt.log.Warn().Err(err).Msg("could not publish an inbox change")
			}
		},
	})
	d.webhooks = webhook.New(webhook.Options{Pool: rt.pool, Log: logging.WithSystem(rt.log, "webhook"), Access: d.access, Projects: d.projects,
		Issues: d.issues, Secrets: rt.secrets(), AllowPrivate: rt.cfg.WebhookAllowPrivate})
	d.vcs = vcs.New(vcs.Options{Pool: rt.pool, Log: logging.WithSystem(rt.log, "vcs"), Access: d.access, Config: d.config, Issues: d.issues,
		Secrets: rt.secrets(), BaseURL: rt.cfg.BaseURL, Client: egress.Client(15*time.Second, rt.cfg.ForgeAllowPrivate)})
	d.sso = sso.New(sso.Options{Pool: rt.pool, Log: logging.WithSystem(rt.log, "sso"), Identity: d.identity, Workspaces: d.workspaces,
		Access: d.access, Secrets: rt.secrets(), BaseURL: rt.cfg.BaseURL, StateSecret: rt.cfg.SessionSecret,
		Client: egress.Client(15*time.Second, rt.cfg.OIDCAllowPrivate)})
	d.identity.SetSignInPolicy(d.sso.Policy)
	return d
}

// api registers every module's routes on /api/v1; the hub pushes events to
// its sockets once running.
func (rt *runtime) api() (func(api *gin.RouterGroup), *realtime.Hub) {
	d := rt.domain()
	auth := identity.NewHTTP(d.identity, rt.cfg.BaseURL.Scheme == "https")
	workspaces := workspace.NewHTTP(d.workspaces, auth)
	roles := access.NewHTTP(d.access)
	projects := project.NewHTTP(d.projects)
	configHTTP := configdef.NewHTTP(d.config)
	issues := issue.NewHTTP(d.issues, d.projects)
	plans := planning.NewHTTP(d.planning)
	searchHTTP := search.NewHTTP(search.New(rt.pool, d.access, d.projects, d.issues))
	viewsHTTP := view.NewHTTP(view.New(rt.pool, d.access, d.projects))
	notifyHTTP := notify.NewHTTP(d.notify)
	webhooksHTTP := webhook.NewHTTP(d.webhooks)
	vcsHTTP := vcs.NewHTTP(d.vcs)
	ssoHTTP := sso.NewHTTP(d.sso, auth, rt.cfg.BaseURL.Scheme == "https")
	reportsHTTP := report.NewHTTP(report.New(rt.pool, d.access, d.config, d.projects, d.issues, d.planning, nil))
	hub := realtime.New(realtime.Options{
		Log: logging.WithSystem(rt.log, "realtime"), Pool: rt.pool, NATS: rt.nc, Stream: eventlog.DefaultStream(rt.cfg.EventsRetention),
		Identity: d.identity, Access: d.access, Workspaces: d.workspaces, Projects: d.projects, Issues: d.issues,
		Origins: []string{rt.cfg.BaseURL.Host},
	})
	return func(api *gin.RouterGroup) {
		vcsHTTP.RegisterHooks(api) // signed by the forge, not authenticated
		api.Use(auth.Authenticate())
		auth.Register(api)
		ssoHTTP.Register(api)
		hub.Register(api)
		notifyHTTP.Register(api)
		workspaces.Register(api, roles.Register, configHTTP.RegisterWorkspace, issues.RegisterWorkspace, searchHTTP.Register, plans.RegisterWorkspace, viewsHTTP.Register, reportsHTTP.Register, notifyHTTP.RegisterWorkspace, webhooksHTTP.Register, vcsHTTP.Register, ssoHTTP.RegisterWorkspace, func(ws *gin.RouterGroup) {
			projects.Register(ws, configHTTP.RegisterProject, issues.RegisterProject, plans.RegisterProject, reportsHTTP.RegisterProject)
		})
	}, hub
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

// checkRelease refuses to run on a database a much newer release migrated
// (docs/operations.md#upgrades).
func (rt *runtime) checkRelease(ctx context.Context) error {
	db := stdlib.OpenDBFromPool(rt.pool)
	defer db.Close()
	last, err := migrations.LastRelease(ctx, db)
	if err != nil {
		return err
	}
	return migrations.CheckRunnable(last, version.Version)
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
