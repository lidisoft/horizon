package cli

import (
	"context"

	"github.com/urfave/cli/v3"
	"golang.org/x/sync/errgroup"

	"github.com/lidisoft/horizon/internal/config"
	"github.com/lidisoft/horizon/internal/logging"
	"github.com/lidisoft/horizon/internal/platform/nats"
	"github.com/lidisoft/horizon/internal/server"
	"github.com/lidisoft/horizon/internal/worker"
)

func workerCommand() *cli.Command {
	return &cli.Command{
		Name:   "worker",
		Usage:  "Run the JetStream consumers: outbox relay, notifications, webhooks…",
		Action: workerAction,
	}
}

func workerAction(ctx context.Context, _ *cli.Command) error {
	rt, err := open(ctx, config.NeedWorker, "worker")
	if err != nil {
		return err
	}
	defer rt.close()

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return server.Run(ctx, rt.cfg.MetricsAddr, rt.metrics.Handler(), logging.WithSystem(rt.log, "metrics"))
	})
	g.Go(func() error { return runWorker(ctx, rt) })
	return g.Wait()
}

func runWorker(ctx context.Context, rt *runtime) error {
	js, err := nats.JetStream(rt.nc)
	if err != nil {
		return err
	}
	return worker.Run(ctx, worker.Deps{
		Log:       logging.WithSystem(rt.log, "worker"),
		JetStream: js,
	})
}
