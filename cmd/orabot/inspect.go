package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/pgrundev/pgbot/internal/config"
	"github.com/pgrundev/pgbot/internal/engine/oracle"
	oraclefindings "github.com/pgrundev/pgbot/internal/engine/oracle/findings"
	oraclerender "github.com/pgrundev/pgbot/internal/engine/oracle/render"
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
		Use:   "inspect <oracle-url>",
		Short: "Collect one read-only Oracle Database report",
		Long: "Connect to one Oracle Database 19c or later service and run only embedded,\n" +
			"statically checked SELECT statements. Prefer $ORACLE_DATABASE_URL so a password\n" +
			"does not appear in shell history or the process argument list.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			return runInspect(command.Context(), app, args, flags)
		},
	}
	values := command.Flags()
	values.StringVar(&flags.format, "format", "text", "output format: text|json")
	values.BoolVar(&flags.json, "json", false, "emit the versioned Oracle report as JSON")
	values.DurationVar(&flags.interval, "interval", time.Second, "gap between Oracle counter samples (min 500ms)")
	values.DurationVar(&flags.timeout, "timeout", 30*time.Second, "total connection and inspection deadline")
	values.BoolVar(&flags.noColor, "no-color", false, "disable ANSI color")
	values.BoolVar(&flags.full, "full", false, "show detailed SQL, lock, object, memory, and parameter sections")
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

	dsn := firstNonEmpty(argument(args, 0), os.Getenv("ORACLE_DATABASE_URL"), os.Getenv("ORABOT_DATABASE_URL"))
	if dsn == "" {
		return fmt.Errorf("no Oracle URL (pass one or set $ORACLE_DATABASE_URL)")
	}

	ctx, cancel := context.WithTimeout(ctx, flags.timeout)
	defer cancel()
	target, err := oracle.Open(ctx, dsn)
	if err != nil {
		return fmt.Errorf("connect: %s", oracle.RedactDSN(err.Error()))
	}
	defer func() { _ = target.Close() }()

	report, err := oracle.Inspect(ctx, target, oracle.InspectOptions{
		Interval: flags.interval, Deadline: flags.timeout, Concurrency: 4,
	})
	if err != nil {
		return fmt.Errorf("inspect: %s", oracle.RedactDSN(err.Error()))
	}
	data, ok := report.Data.(*oracle.Data)
	if !ok || data == nil {
		return fmt.Errorf("inspect: Oracle report returned %T engine_data", report.Data)
	}
	report.Findings = applyIgnores(oraclefindings.Compute(data), flags.ignore)

	switch flags.format {
	case "json":
		err = oraclerender.JSON(app.stdout, report)
	default:
		err = oraclerender.Terminal(app.stdout, report, oraclerender.Options{
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
