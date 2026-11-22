// Package bootstrap is the composition root: the only package that knows every
// adapter. It reads configuration, builds driven adapters, injects them into
// the core services, and exposes the driving adapters' dependencies.
package bootstrap

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/Tracer-Cloud/opensre/go/internal/adapters/primary/cli"
	"github.com/Tracer-Cloud/opensre/go/internal/adapters/primary/httpapi"
	"github.com/Tracer-Cloud/opensre/go/internal/adapters/primary/worker"
	"github.com/Tracer-Cloud/opensre/go/internal/adapters/secondary/integrations/datadog"
	"github.com/Tracer-Cloud/opensre/go/internal/adapters/secondary/integrations/grafana"
	"github.com/Tracer-Cloud/opensre/go/internal/adapters/secondary/llm/anthropic"
	queuemem "github.com/Tracer-Cloud/opensre/go/internal/adapters/secondary/queue/memory"
	repomem "github.com/Tracer-Cloud/opensre/go/internal/adapters/secondary/repository/memory"
	"github.com/Tracer-Cloud/opensre/go/internal/adapters/secondary/system"
	"github.com/Tracer-Cloud/opensre/go/internal/core/ports"
	"github.com/Tracer-Cloud/opensre/go/internal/core/services/agent"
	"github.com/Tracer-Cloud/opensre/go/internal/core/services/intake"
	"github.com/Tracer-Cloud/opensre/go/internal/core/services/investigation"
	"github.com/Tracer-Cloud/opensre/go/internal/platform/config"
)

// Overrides replace default adapters (tests, embedding).
type Overrides struct {
	LLM        ports.LLM
	Events     ports.EventSink
	HTTPClient *http.Client
}

// App is a fully wired process.
type App struct {
	Config   config.Config
	Registry *agent.Registry
	Service  *investigation.Service
	Intake   *intake.Service
	Queue    *queuemem.Inbox
	Handler  http.Handler
	log      *slog.Logger
}

// Build wires the application.
func Build(cfg config.Config, log *slog.Logger, o Overrides) (*App, error) {
	tools := integrationTools(cfg, o.HTTPClient)
	registry, err := agent.NewRegistry(tools...)
	if err != nil {
		return nil, err
	}
	llm := o.LLM
	if llm == nil {
		llm = anthropic.New(anthropic.Config{
			Model: cfg.Model, Effort: cfg.Effort, DisableFallbacks: cfg.DisableFallbacks,
		})
	}
	events := o.Events
	if events == nil {
		events = noEvents{}
	}
	runCfg := agent.DefaultConfig()
	runCfg.MaxIterations = cfg.MaxIterations
	runner := agent.NewRunner(llm, registry, runCfg, agent.WithEvents(events))

	clock := system.Clock{}
	queue := queuemem.NewInbox(queuemem.DefaultCapacity)
	svc := investigation.New(runner, repomem.New(), clock, system.IDs{}, log)
	in := intake.New(queue, clock)

	return &App{
		Config:   cfg,
		Registry: registry,
		Service:  svc,
		Intake:   in,
		Queue:    queue,
		log:      log,
		Handler: httpapi.New(httpapi.Deps{
			Intake: in, Investigator: svc, Assistant: svc, Tokens: cfg.ListenerTokens, Log: log,
		}),
	}, nil
}

// integrationTools registers each integration whose credentials are set.
func integrationTools(cfg config.Config, hc *http.Client) []ports.Tool {
	var tools []ports.Tool
	dd := datadog.Config{APIKey: cfg.DatadogAPIKey, AppKey: cfg.DatadogAppKey, Site: cfg.DatadogSite}
	if dd.Configured() {
		tools = append(tools, datadog.Tools(datadog.NewClient(dd, hc))...)
	}
	gf := grafana.Config{InstanceURL: cfg.GrafanaURL, ReadToken: cfg.GrafanaToken, LokiUID: cfg.GrafanaLokiUID}
	if gf.Configured() {
		tools = append(tools, grafana.Tools(grafana.NewClient(gf, hc))...)
	}
	return tools
}

// CLIDeps returns what the CLI adapter drives.
func (a *App) CLIDeps() cli.Deps {
	return cli.Deps{
		Investigator: a.Service,
		Assistant:    a.Service,
		Catalog:      a.Registry,
		Serve:        a.Serve,
		DefaultAddr:  a.Config.Addr,
	}
}

// Serve runs the HTTP API and the alert worker until ctx is cancelled.
func (a *App) Serve(ctx context.Context, addr string) error {
	srv := &http.Server{Addr: addr, Handler: a.Handler, ReadHeaderTimeout: 10 * time.Second}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	w := &worker.Worker{Queue: a.Queue, Investigator: a.Service, Concurrency: a.Config.Workers, Log: a.log}
	done := make(chan struct{})
	go func() { w.Run(ctx); close(done) }()

	errc := make(chan error, 1)
	go func() {
		a.log.Info("listening", "addr", addr, "tools", a.Registry.Len())
		errc <- srv.ListenAndServe()
	}()

	var err error
	select {
	case <-ctx.Done():
	case err = <-errc:
	}
	cancel()
	shutdownCtx, stop := context.WithTimeout(context.Background(), 10*time.Second)
	defer stop()
	_ = srv.Shutdown(shutdownCtx)
	<-done
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

type noEvents struct{}

func (noEvents) Emit(context.Context, ports.Event) {}
