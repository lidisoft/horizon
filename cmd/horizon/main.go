// Command horizon is the single Horizon binary: serve, worker, migrate, admin.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/lidisoft/horizon/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	err := cli.New().Run(ctx, os.Args)
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, "horizon:", err)
		os.Exit(1)
	}
}
