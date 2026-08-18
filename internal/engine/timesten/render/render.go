// Package render writes TimesTen reports without depending on PostgreSQL
// presentation types.
package render

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/pgrundev/pgbot/internal/engine"
	"github.com/pgrundev/pgbot/internal/engine/timesten"
	"github.com/pgrundev/pgbot/internal/model"
)

// Options controls TimesTen terminal presentation only.
type Options struct {
	Color bool
	Full  bool
}

type styler struct{ color bool }

func (s styler) render(code, value string, bold bool) string {
	if !s.color {
		return value
	}
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(code))
	if bold {
		style = style.Bold(true)
	}
	return style.Render(value)
}

func (s styler) head(value string) string { return s.render("39", value, true) }
func (s styler) good(value string) string { return s.render("42", value, false) }
func (s styler) warn(value string) string { return s.render("208", value, true) }
func (s styler) crit(value string) string { return s.render("196", value, true) }
func (s styler) dim(value string) string  { return s.render("240", value, false) }

// JSON writes the common envelope and TimesTen-owned engine_data.
func JSON(writer io.Writer, report *engine.Report) error {
	if report == nil {
		return fmt.Errorf("render TimesTen JSON: nil report")
	}
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(report)
}

// Terminal writes a findings-first TimesTen report.
func Terminal(writer io.Writer, report *engine.Report, options Options) error {
	data, err := reportData(report)
	if err != nil {
		return err
	}
	style := styler{color: options.Color}
	var output strings.Builder
	fmt.Fprintf(&output, "%s · %s · TimesTen %s · %s · %s · %.1fs sample\n\n",
		style.good("connected"), style.head(report.Target.Database), report.Target.Version,
		style.dim("Classic client/server"), style.dim("SELECT-only"), report.Window.SampleSeconds)
	renderFindings(&output, style, report.Findings, options.Full)
	renderHealth(&output, style, data.Health)
	renderConnections(&output, style, data.Connections)
	renderSpace(&output, style, data.Space)
	renderPersistence(&output, style, data.Persistence)
	renderReplication(&output, style, data.Replication)
	if options.Full {
		renderLocks(&output, style, data.Locks)
		renderTopSQL(&output, style, data.TopSQL)
		renderTables(&output, style, data.Tables)
		renderIndexes(&output, style, data.Indexes)
		renderConfiguration(&output, style, data.Configuration)
	} else {
		fmt.Fprintln(&output, style.dim("Run with --full for lock counters, TTStats SQL, schema, index, and capability details."))
	}
	_, err = io.WriteString(writer, output.String())
	return err
}

func reportData(report *engine.Report) (*timesten.Data, error) {
	if report == nil {
		return nil, fmt.Errorf("render TimesTen terminal: nil report")
	}
	if report.Engine != engine.TimesTen {
		return nil, fmt.Errorf("render TimesTen terminal: report engine is %q", report.Engine)
	}
	data, ok := report.Data.(*timesten.Data)
	if !ok || data == nil {
		return nil, fmt.Errorf("render TimesTen terminal: engine_data is %T", report.Data)
	}
	return data, nil
}

func renderFindings(output *strings.Builder, style styler, findings []model.Finding, full bool) {
	var visible, suppressed []model.Finding
	for _, finding := range findings {
		if finding.Suppressed && finding.Severity != model.SeverityCritical {
			suppressed = append(suppressed, finding)
			continue
		}
		visible = append(visible, finding)
	}
	if len(visible) == 0 {
		fmt.Fprintln(output, style.good("✓ no TimesTen findings — nothing stood out"))
		fmt.Fprintln(output)
	} else {
		critical, warnings := 0, 0
		for _, finding := range visible {
			switch finding.Severity {
			case model.SeverityCritical:
				critical++
			case model.SeverityWarn:
				warnings++
			}
		}
		fmt.Fprintln(output, style.head(fmt.Sprintf("FINDINGS · %d critical · %d warning", critical, warnings)))
		for _, finding := range visible {
			badge := strings.ToUpper(finding.Severity)
			switch finding.Severity {
			case model.SeverityCritical:
				badge = style.crit(badge)
			case model.SeverityWarn:
				badge = style.warn(badge)
			default:
				badge = style.dim(badge)
			}
			mark := ""
			if finding.Suppressed {
				mark = " " + style.dim("[suppressed]")
			}
			fmt.Fprintf(output, "  %s  %s%s\n", badge, finding.Title, mark)
			if full {
				fmt.Fprintf(output, "    %s\n", finding.Detail)
				for index, evidence := range finding.Evidence {
					if index == 5 {
						fmt.Fprintf(output, "    %s\n", style.dim(fmt.Sprintf("… %d more evidence rows in JSON", len(finding.Evidence)-index)))
						break
					}
					fmt.Fprintf(output, "    %s\n", style.dim("• "+evidence))
				}
				if finding.Remediation != "" {
					fmt.Fprintf(output, "    %s\n", style.dim("→ "+finding.Remediation))
				}
			}
		}
		fmt.Fprintln(output)
	}
	if full && len(suppressed) > 0 {
		fmt.Fprintln(output, style.dim(fmt.Sprintf("SUPPRESSED · %d", len(suppressed))))
		for _, finding := range suppressed {
			fmt.Fprintln(output, "  "+style.dim(finding.Title+" — "+finding.SuppressionReason))
		}
		fmt.Fprintln(output)
	}
}

func renderHealth(output *strings.Builder, style styler, health *timesten.Health) {
	if health == nil || !section(output, style, "health", health.Section) {
		return
	}
	fmt.Fprintf(output, "  transactions %.2f/s · commits %.2f/s · rollbacks %.2f/s · rollback ratio %.1f%%\n",
		health.TransactionsPerSec, health.CommitsPerSec, health.RollbacksPerSec, health.RollbackRatio*100)
	fmt.Fprintf(output, "  SELECT %.2f/s · writes %.2f/s · log %s/s · checkpoints %.2f/s\n\n",
		health.SelectsPerSec, health.WritesPerSec, humanBytes(int64(health.LogBytesPerSec)), health.CheckpointsPerSec)
}

func renderConnections(output *strings.Builder, style styler, connections *timesten.Connections) {
	if connections == nil || !section(output, style, "connections", connections.Section) {
		return
	}
	fmt.Fprintf(output, "  %d current · %d established · %d disconnected · %.2f new/s\n\n",
		connections.Current, connections.Established, connections.Disconnected, connections.EstablishedPerSec)
}

func renderSpace(output *strings.Builder, style styler, space *timesten.Space) {
	if space == nil || !section(output, style, "space", space.Section) {
		return
	}
	for _, item := range []struct {
		name string
		pool timesten.SpacePool
	}{{"permanent", space.Permanent}, {"temporary", space.Temporary}} {
		fmt.Fprintf(output, "  %-10s %5.1f%% used · %s / %s · high water %.1f%%\n",
			item.name, item.pool.UsedRatio*100, humanBytes(item.pool.UsedBytes),
			humanBytes(item.pool.AllocatedBytes), item.pool.HighWaterRatio*100)
	}
	fmt.Fprintln(output)
}

func renderPersistence(output *strings.Builder, style styler, persistence *timesten.Persistence) {
	if persistence == nil || !section(output, style, "persistence", persistence.Section) {
		return
	}
	recovery := style.good("not required")
	if persistence.RequiredRecovery {
		recovery = style.crit("required")
	}
	fmt.Fprintf(output, "  recovery %s · log files %d..%d · replication hold %d\n\n",
		recovery, persistence.FirstLogFile, persistence.LastLogFile, persistence.ReplicationHold)
}

func renderReplication(output *strings.Builder, style styler, replication *timesten.Replication) {
	if replication == nil || !section(output, style, "replication", replication.Section) {
		return
	}
	if len(replication.Definitions) == 0 {
		fmt.Fprintln(output, "  no Classic replication scheme configured")
	} else {
		fmt.Fprintf(output, "  %d scheme definitions · %d peer tracks\n", len(replication.Definitions), len(replication.Peers))
		for _, peer := range replication.Peers {
			state := "unknown"
			if peer.State != nil {
				state = fmt.Sprintf("%d", *peer.State)
			}
			latency := "unknown"
			if peer.Latency != nil {
				latency = fmt.Sprintf("%.3fs", *peer.Latency)
			}
			fmt.Fprintf(output, "  %s.%s subscriber %d track %d · state %s · latency %s\n",
				peer.Owner, peer.Name, peer.SubscriberID, peer.TrackID, state, latency)
		}
	}
	fmt.Fprintln(output)
}

func renderLocks(output *strings.Builder, style styler, locks *timesten.Locks) {
	if locks == nil || !section(output, style, "locks", locks.Section) {
		return
	}
	fmt.Fprintf(output, "  deadlocks %d cumulative / %.2f/s · timeouts %d / %.2f/s · waited grants %d / %.2f/s\n\n",
		locks.Deadlocks, locks.DeadlocksPerSec, locks.Timeouts, locks.TimeoutsPerSec, locks.WaitGrants, locks.WaitGrantsPerSec)
}

func renderTopSQL(output *strings.Builder, style styler, topSQL *timesten.TopSQL) {
	if topSQL == nil || !section(output, style, "top SQL", topSQL.Section) {
		return
	}
	for _, statement := range topSQL.Statements {
		fmt.Fprintf(output, "  %s · max %.3fs · last %.3fs · %d executions · %s\n",
			statement.SQLHash, statement.MaxSeconds, statement.LastSeconds, statement.Executions,
			truncate(timesten.ScrubQueryText(statement.SampleText), 90))
	}
	if len(topSQL.Statements) == 0 {
		fmt.Fprintln(output, "  no TTStats statements visible")
	}
	fmt.Fprintln(output)
}

func renderTables(output *strings.Builder, style styler, tables *timesten.Tables) {
	if tables == nil || !section(output, style, "tables", tables.Section) {
		return
	}
	missing := 0
	for _, table := range tables.Rows {
		if table.MissingStatistics {
			missing++
		}
	}
	fmt.Fprintf(output, "  %d application tables · %d non-empty without statistics%s\n\n",
		len(tables.Rows), missing, truncatedMark(tables.Truncated))
}

func renderIndexes(output *strings.Builder, style styler, indexes *timesten.Indexes) {
	if indexes == nil || !section(output, style, "indexes", indexes.Section) {
		return
	}
	hash, rangeCount := 0, 0
	for _, index := range indexes.Rows {
		switch index.TypeCode {
		case 3:
			rangeCount++
		default:
			hash++
		}
	}
	fmt.Fprintf(output, "  %d application indexes · %d range · %d other%s\n\n",
		len(indexes.Rows), rangeCount, hash, truncatedMark(indexes.Truncated))
}

func renderConfiguration(output *strings.Builder, style styler, configuration *timesten.Configuration) {
	if configuration == nil {
		return
	}
	section(output, style, "configuration", configuration.Section)
}

func section(output *strings.Builder, style styler, name string, value engine.Section) bool {
	fmt.Fprintf(output, "%s  %s\n", style.head(strings.ToUpper(name)), style.dim(value.Exactness))
	if value.Reason != "" {
		fmt.Fprintln(output, "  "+style.dim(value.Reason))
	}
	if value.Exactness == model.ExactnessUnavailable {
		fmt.Fprintln(output)
		return false
	}
	if value.Exactness == model.ExactnessReset {
		fmt.Fprintln(output)
		return false
	}
	return true
}

func truncate(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) <= limit {
		return value
	}
	return value[:limit-1] + "…"
}

func humanBytes(value int64) string {
	const unit = int64(1024)
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}
	divisor, exponent := unit, 0
	for n := value / unit; n >= unit && exponent < 5; n /= unit {
		divisor *= unit
		exponent++
	}
	return fmt.Sprintf("%.1f %ciB", float64(value)/float64(divisor), "KMGTPE"[exponent])
}

func truncatedMark(truncated bool) string {
	if truncated {
		return " · first 500 only"
	}
	return ""
}
