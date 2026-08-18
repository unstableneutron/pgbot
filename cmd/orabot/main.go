// Command orabot provides read-only Oracle Database inspection.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/pgrundev/pgbot/internal/engine/oracle"
	"github.com/pgrundev/pgbot/internal/model"
	"github.com/spf13/cobra"
)

const (
	exitClean    = 0
	exitWarn     = 1
	exitCritical = 2
	exitFailure  = 3
	exitUsage    = 64
)

var version = "dev"

type application struct {
	stdout io.Writer
	status int
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := execute(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

func execute(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	app := &application{stdout: stdout}
	root := newRootCommand(app)
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)

	enteredRun := false
	root.PersistentPreRun = func(*cobra.Command, []string) { enteredRun = true }
	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(stderr, "orabot: "+oracle.RedactDSN(err.Error()))
		var usage usageError
		if !enteredRun || errors.As(err, &usage) {
			return exitUsage
		}
		return exitFailure
	}
	return app.status
}

func newRootCommand(app *application) *cobra.Command {
	root := &cobra.Command{
		Use:           "orabot",
		Short:         "Read-only Oracle Database observability",
		Long:          "orabot connects read-only to Oracle Database 19c or later and reports health, activity, SQL, storage, schema, configuration, and recovery evidence.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
	}
	root.AddCommand(newInspectCommand(app))
	return root
}

type usageError struct{ error }

func usageErrf(format string, values ...any) error {
	return usageError{fmt.Errorf(format, values...)}
}

func findingExitCode(findings []model.Finding, failOn string) int {
	threshold := severityRank(failOn)
	if threshold == 0 {
		return exitClean
	}
	highest := 0
	for _, finding := range findings {
		if finding.Suppressed {
			continue
		}
		rank := severityRank(finding.Severity)
		if rank > highest {
			highest = rank
		}
	}
	if highest < threshold {
		return exitClean
	}
	if highest >= severityRank(model.SeverityCritical) {
		return exitCritical
	}
	return exitWarn
}

func severityRank(severity string) int {
	switch severity {
	case model.SeverityCritical:
		return 3
	case model.SeverityWarn:
		return 2
	case model.SeverityInfo:
		return 1
	default:
		return 0
	}
}
