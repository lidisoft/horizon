package cli

import (
	"context"
	"time"

	"github.com/urfave/cli/v3"
	"golang.org/x/sync/errgroup"

	"github.com/lidisoft/horizon/internal/config"
	"github.com/lidisoft/horizon/internal/eventlog"
	"github.com/lidisoft/horizon/internal/issue"
	"github.com/lidisoft/horizon/internal/logging"
	"github.com/lidisoft/horizon/internal/platform/nats"
	"github.com/lidisoft/horizon/internal/report"
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
	d := rt.domain()
	reports := report.NewProjector(rt.pool, logging.WithSystem(rt.log, "reports"))
	return worker.Run(ctx, worker.Deps{
		Log:           logging.WithSystem(rt.log, "worker"),
		Pool:          rt.pool,
		JetStream:     js,
		Stream:        eventlog.DefaultStream(rt.cfg.EventsRetention),
		Metrics:       rt.metrics,
		DeleteObjects: rt.s3.Delete,
		Consumers:     []eventlog.Consumer{d.planning.PolicyConsumer(), reports.Consumer(), d.notify.Consumer()},
		Jobs: []worker.Job{
			{Name: "identity-cleanup", Every: time.Hour, Run: rt.identity().Cleanup},
			// The consumer wakes the projector on each event; this catches
			// up whatever it missed (a rebuild, events of a stopped relay).
			{Name: "reports", Every: time.Minute, Run: func(ctx context.Context) error {
				_, err := reports.CatchUp(ctx)
				return err
			}},
			{Name: "email", Every: time.Minute, Run: d.notify.SendDue},
			{Name: "digest", Every: 5 * time.Minute, Run: d.notify.SendDigests},
			{Name: "describe", Every: 10 * time.Second, Run: func(ctx context.Context) error {
				_, err := d.issues.DescribeQuiet(ctx)
				return err
			}},
			{Name: "upload-expiry", Every: time.Hour, Run: func(ctx context.Context) error {
				return issue.ExpireUploads(ctx, rt.pool, time.Now().Add(-issue.UploadExpiry))
			}},
		},
	})
}
