package cli

import (
	"context"

	"github.com/urfave/cli/v3"
	"golang.org/x/sync/errgroup"

	"github.com/lidisoft/horizon/internal/config"
	"github.com/lidisoft/horizon/internal/logging"
	"github.com/lidisoft/horizon/internal/server"
	"github.com/lidisoft/horizon/web"
)

func serveCommand() *cli.Command {
	return &cli.Command{
		Name:  "serve",
		Usage: "Serve the HTTP API, WebSockets and the web app",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "with-worker",
				Usage: "also run the worker in this process (small installs)",
			},
		},
		Action: serveAction,
	}
}

func serveAction(ctx context.Context, cmd *cli.Command) error {
	rt, err := open(ctx, config.NeedServe, "serve")
	if err != nil {
		return err
	}
	defer rt.close()

	webApp, err := server.NewWeb(web.FS(), rt.s3.PublicOrigin)
	if err != nil {
		return err
	}
	// Browsers upload attachments straight to the bucket.
	if err := rt.s3.AllowOrigin(ctx, rt.cfg.BaseURL.Scheme+"://"+rt.cfg.BaseURL.Host); err != nil {
		rt.log.Warn().Err(err).Msg("could not allow the app's origin in the bucket's CORS rules; attachment uploads may fail")
	}
	engine := server.NewEngine(server.Deps{
		Log:     rt.log,
		Checks:  rt.checks(),
		Metrics: rt.metrics,
		Web:     webApp,
		API:     rt.api(),

		TrustedProxies: rt.cfg.TrustedProxies,
	})

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return server.Run(ctx, rt.cfg.HTTPAddr, engine, logging.WithSystem(rt.log, "http"))
	})
	g.Go(func() error {
		return server.Run(ctx, rt.cfg.MetricsAddr, rt.metrics.Handler(), logging.WithSystem(rt.log, "metrics"))
	})
	if cmd.Bool("with-worker") {
		g.Go(func() error { return runWorker(ctx, rt) })
	}
	return g.Wait()
}
