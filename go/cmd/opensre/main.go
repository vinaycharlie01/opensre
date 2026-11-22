// Command opensre is the OpenSRE AI SRE agent.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Tracer-Cloud/opensre/go/internal/adapters/primary/cli"
	"github.com/Tracer-Cloud/opensre/go/internal/adapters/secondary/progress"
	"github.com/Tracer-Cloud/opensre/go/internal/bootstrap"
	"github.com/Tracer-Cloud/opensre/go/internal/platform/config"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg, err := config.Load(nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(2)
	}
	app, err := bootstrap.Build(cfg, log, bootstrap.Overrides{Events: progress.NewWriter(os.Stderr)})
	if err != nil {
		fmt.Fprintln(os.Stderr, "startup:", err)
		os.Exit(1)
	}
	os.Exit(cli.Run(ctx, os.Args[1:], os.Stdout, os.Stderr, app.CLIDeps()))
}
