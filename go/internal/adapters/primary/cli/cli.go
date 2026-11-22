// Package cli is the command-line driving adapter.
package cli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/alert"
	"github.com/Tracer-Cloud/opensre/go/internal/core/domain/investigation"
	"github.com/Tracer-Cloud/opensre/go/internal/core/ports"
)

// Deps are the use cases and host hooks the CLI drives.
type Deps struct {
	Investigator ports.Investigator
	Assistant    ports.Assistant
	Catalog      ports.ToolCatalog
	// Serve runs the HTTP API and alert worker until ctx is done.
	Serve func(ctx context.Context, addr string) error
	// DefaultAddr is the serve address when --addr is not given.
	DefaultAddr string
}

const usage = `OpenSRE — AI SRE agent

Usage:
  opensre investigate --text TEXT [--name NAME] [--severity SEV] [--source SRC] [--json]
  opensre ask QUESTION
  opensre serve [--addr HOST:PORT]
  opensre tools

Environment:
  ANTHROPIC_API_KEY (or an ant auth profile)  Claude credentials
  OPENSRE_MODEL      optional, default claude-opus-5-5
  OPENSRE_EFFORT     optional, low|medium|high|xhigh|max, default high
  DD_API_KEY, DD_APP_KEY, DD_SITE              optional, enable Datadog tools
  GRAFANA_INSTANCE_URL, GRAFANA_LOKI_DATASOURCE_UID, GRAFANA_READ_TOKEN
                                               optional, enable Grafana tools
  OPENSRE_GATEWAY_LISTENER_TOKEN               optional, required for non-loopback HTTP callers
`

// Run executes one command and returns the process exit code.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer, d Deps) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	var err error
	switch args[0] {
	case "investigate":
		err = investigate(ctx, args[1:], stdout, d)
	case "ask":
		err = ask(ctx, args[1:], stdout, d)
	case "serve":
		err = serve(ctx, args[1:], d)
	case "tools":
		listTools(stdout, d)
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n%s", args[0], usage)
		return 2
	}
	if err != nil {
		fmt.Fprintln(stderr, "error:", err)
		if _, ok := err.(usageError); ok {
			return 2
		}
		return 1
	}
	return 0
}

type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

func investigate(ctx context.Context, args []string, out io.Writer, d Deps) error {
	fs := flag.NewFlagSet("investigate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	text := fs.String("text", "", "alert text (required)")
	name := fs.String("name", "", "alert name")
	severity := fs.String("severity", "", "alert severity")
	source := fs.String("source", "", "alert source, e.g. datadog or grafana")
	asJSON := fs.Bool("json", false, "print the investigation as JSON")
	if err := fs.Parse(args); err != nil {
		return usageError{"investigate: " + err.Error()}
	}
	a := alert.Alert{Text: *text, Name: *name, Severity: alert.Severity(*severity), Source: *source}.Normalize()
	// Validate before any model call, with the same rules as HTTP intake.
	if err := a.Validate(); err != nil {
		return usageError{"investigate: --text: " + err.Error()}
	}
	inv, err := d.Investigator.Investigate(ctx, a)
	if err != nil {
		return err
	}
	if *asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(inv)
	}
	printReport(out, inv)
	if inv.Status == investigation.StatusFailed {
		return fmt.Errorf("investigation failed: %s", inv.Failure)
	}
	return nil
}

func printReport(out io.Writer, inv investigation.Investigation) {
	fmt.Fprintf(out, "\n%s\n\n", inv.Report)
	if len(inv.Evidence) > 0 {
		fmt.Fprintln(out, "Evidence:")
		for _, e := range inv.Evidence {
			fmt.Fprintf(out, "  [%s] %s — %s\n", e.ID, e.Label, e.Summary)
		}
	}
	fmt.Fprintf(out, "\n%s · %s · %d tool calls · %d in / %d out tokens\n",
		inv.ID, inv.Outcome, inv.ToolCalls, inv.Usage.InputTokens, inv.Usage.OutputTokens)
}

func ask(ctx context.Context, args []string, out io.Writer, d Deps) error {
	q := strings.TrimSpace(strings.Join(args, " "))
	if q == "" {
		return usageError{"ask: QUESTION is required"}
	}
	ans, err := d.Assistant.Ask(ctx, q)
	if err != nil {
		return err
	}
	fmt.Fprintln(out, ans.Text)
	return nil
}

func serve(ctx context.Context, args []string, d Deps) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	addr := fs.String("addr", d.DefaultAddr, "listen address")
	if err := fs.Parse(args); err != nil {
		return usageError{"serve: " + err.Error()}
	}
	return d.Serve(ctx, *addr)
}

func listTools(out io.Writer, d Deps) {
	specs := d.Catalog.Specs()
	if len(specs) == 0 {
		fmt.Fprintln(out, "No tools configured. Set Datadog or Grafana environment variables (see `opensre help`).")
		return
	}
	tw := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tSOURCE\tSIDE EFFECT")
	for _, s := range specs {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", s.Name, s.Source, s.SideEffect)
	}
	tw.Flush()
}
