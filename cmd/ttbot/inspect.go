//go:build timesten && cgo

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/pgrundev/pgbot/internal/config"
	"github.com/pgrundev/pgbot/internal/engine/timesten"
	timestenfindings "github.com/pgrundev/pgbot/internal/engine/timesten/findings"
	timestenrender "github.com/pgrundev/pgbot/internal/engine/timesten/render"
	"github.com/pgrundev/pgbot/internal/model"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

type inspectFlags struct {
	format   string
	json     bool
	interval time.Duration
	timeout  time.Duration
	noColor  bool
	full     bool
	failOn   string
	ignore   []string
}

func newInspectCommand(app *application) *cobra.Command {
	var flags inspectFlags
	command := &cobra.Command{
		Use:   "inspect <odbc-connection-string>",
		Short: "Collect one SELECT-only TimesTen report",
		Long: "Connect to TimesTen 22.1 Classic through an explicit client/server ODBC string.\n" +
			"The string must include TTC_SERVER and TTC_SERVER_DSN. Prefer\n" +
			"$TIMESTEN_CONNECTION_STRING so a password does not appear in shell history.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			return runInspect(command.Context(), app, args, flags)
		},
	}
	values := command.Flags()
	values.StringVar(&flags.format, "format", "text", "output format: text|json")
	values.BoolVar(&flags.json, "json", false, "emit the versioned TimesTen report as JSON")
	values.DurationVar(&flags.interval, "interval", time.Second, "gap between TimesTen counter samples (min 500ms)")
	values.DurationVar(&flags.timeout, "timeout", 30*time.Second, "total connection and inspection deadline")
	values.BoolVar(&flags.noColor, "no-color", false, "disable ANSI color")
	values.BoolVar(&flags.full, "full", false, "show lock, TTStats SQL, schema, index, and capability details")
	values.StringVar(&flags.failOn, "fail-on", "warn", "finding exit threshold: critical|warn|info|none")
	values.StringArrayVar(&flags.ignore, "ignore", nil, "suppress a finding for this run: finding[:object] (repeatable)")
	return command
}

func runInspect(ctx context.Context, app *application, args []string, flags inspectFlags) error {
	if flags.json && flags.format == "text" {
		flags.format = "json"
	}
	if flags.format != "text" && flags.format != "json" {
		return usageErrf("--format must be text|json, got %q", flags.format)
	}
	if severityRank(flags.failOn) == 0 && flags.failOn != "none" {
		return usageErrf("--fail-on must be critical|warn|info|none, got %q", flags.failOn)
	}
	if flags.interval < 500*time.Millisecond {
		return usageErrf("--interval must be at least 500ms, got %s", flags.interval)
	}
	if flags.timeout <= flags.interval {
		return usageErrf("--timeout must be greater than --interval")
	}

	dsn := firstNonEmpty(argument(args, 0), os.Getenv("TIMESTEN_CONNECTION_STRING"), os.Getenv("TTBOT_CONNECTION_STRING"))
	if dsn == "" {
		return fmt.Errorf("no TimesTen connection string (pass one or set $TIMESTEN_CONNECTION_STRING)")
	}

	ctx, cancel := context.WithTimeout(ctx, flags.timeout)
	defer cancel()
	target, err := timesten.Open(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connect: %s", timesten.RedactDSN(err.Error()))
	}
	defer func() { _ = target.Close() }()

	report, err := timesten.Inspect(ctx, target, timesten.InspectOptions{
		Interval: flags.interval, Deadline: flags.timeout, Concurrency: 4,
	})
	if err != nil {
		return fmt.Errorf("inspect: %s", timesten.RedactDSN(err.Error()))
	}
	data, ok := report.Data.(*timesten.Data)
	if !ok || data == nil {
		return fmt.Errorf("inspect: TimesTen report returned %T engine_data", report.Data)
	}
	report.Findings = applyIgnores(timestenfindings.Compute(data), flags.ignore)

	switch flags.format {
	case "json":
		err = timestenrender.JSON(app.stdout, report)
	default:
		err = timestenrender.Terminal(app.stdout, report, timestenrender.Options{
			Color: useColor(flags.noColor, app.stdout), Full: flags.full,
		})
	}
	if err != nil {
		return err
	}
	app.status = findingExitCode(report.Findings, flags.failOn)
	return nil
}

func applyIgnores(findings []model.Finding, specs []string) []model.Finding {
	configuration := &config.Config{}
	configuration.AddInlineIgnores(specs)
	return configuration.Apply(findings, time.Now())
}

func argument(args []string, index int) string {
	if index < len(args) {
		return args[index]
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func useColor(disabled bool, output io.Writer) bool {
	if disabled || os.Getenv("NO_COLOR") != "" {
		return false
	}
	file, ok := output.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}
