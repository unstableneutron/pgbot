// Package render writes Oracle Database reports without depending on the
// PostgreSQL presentation model.
package render

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/pgrundev/pgbot/internal/engine"
	"github.com/pgrundev/pgbot/internal/engine/oracle"
	"github.com/pgrundev/pgbot/internal/model"
)

// Options controls Oracle terminal presentation only.
type Options struct {
	Color bool
	Full  bool
}

type styler struct{ color bool }

func (s styler) render(code, text string, bold bool) string {
	if !s.color {
		return text
	}
	style := lipgloss.NewStyle().Foreground(lipgloss.Color(code))
	if bold {
		style = style.Bold(true)
	}
	return style.Render(text)
}

func (s styler) head(text string) string { return s.render("39", text, true) }
func (s styler) good(text string) string { return s.render("42", text, false) }
func (s styler) warn(text string) string { return s.render("208", text, true) }
func (s styler) crit(text string) string { return s.render("196", text, true) }
func (s styler) dim(text string) string  { return s.render("240", text, false) }

// JSON writes the versioned common envelope and Oracle-owned engine_data.
func JSON(w io.Writer, report *engine.Report) error {
	if report == nil {
		return fmt.Errorf("render Oracle JSON: nil report")
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	encoder.SetEscapeHTML(false)
	return encoder.Encode(report)
}

// Terminal writes a findings-first Oracle report.
func Terminal(w io.Writer, report *engine.Report, opts Options) error {
	data, err := reportData(report)
	if err != nil {
		return err
	}
	style := styler{color: opts.Color}
	var out strings.Builder

	target := report.Target.Database
	if report.Target.Container != "" && !strings.EqualFold(report.Target.Container, report.Target.Database) {
		target += "/" + report.Target.Container
	}
	fmt.Fprintf(&out, "%s · %s · oracle %s · %s · %.1fs sample\n\n",
		style.good("connected"), style.head(target), report.Target.Version, style.dim("read-only"), report.Window.SampleSeconds)
	renderFindings(&out, style, report.Findings, opts.Full)
	renderHealth(&out, style, data.Health)
	renderSessions(&out, style, data.Sessions)
	renderStorage(&out, style, data.Storage)
	renderResources(&out, style, data.Resources)
	renderRecovery(&out, style, data.Recovery)
	if opts.Full {
		renderLocks(&out, style, data.Locks)
		renderTopSQL(&out, style, data.TopSQL)
		renderMemory(&out, style, data.Memory)
		renderTables(&out, style, data.Tables)
		renderIndexes(&out, style, data.Indexes)
		renderConfiguration(&out, style, data.Configuration)
	} else {
		fmt.Fprintln(&out, style.dim("Run with --full for SQL, lock, object, memory, and parameter details."))
	}

	_, err = io.WriteString(w, out.String())
	return err
}

func reportData(report *engine.Report) (*oracle.Data, error) {
	if report == nil {
		return nil, fmt.Errorf("render Oracle terminal: nil report")
	}
	if report.Engine != engine.Oracle {
		return nil, fmt.Errorf("render Oracle terminal: report engine is %q", report.Engine)
	}
	data, ok := report.Data.(*oracle.Data)
	if !ok || data == nil {
		return nil, fmt.Errorf("render Oracle terminal: engine_data is %T", report.Data)
	}
	return data, nil
}

func renderFindings(out *strings.Builder, style styler, all []model.Finding, full bool) {
	var visible, suppressed []model.Finding
	for _, finding := range all {
		if finding.Suppressed && finding.Severity != model.SeverityCritical {
			suppressed = append(suppressed, finding)
			continue
		}
		visible = append(visible, finding)
	}
	if len(visible) == 0 {
		fmt.Fprintln(out, style.good("✓ no Oracle findings — nothing stood out"))
		fmt.Fprintln(out)
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
		fmt.Fprintf(out, "%s\n", style.head(fmt.Sprintf("FINDINGS · %d critical · %d warning", critical, warnings)))
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
			suppressedMark := ""
			if finding.Suppressed {
				suppressedMark = " " + style.dim("[suppressed]")
			}
			fmt.Fprintf(out, "  %s  %s%s\n", badge, finding.Title, suppressedMark)
			if full {
				fmt.Fprintf(out, "    %s\n", finding.Detail)
				for i, evidence := range finding.Evidence {
					if i == 5 {
						fmt.Fprintf(out, "    %s\n", style.dim(fmt.Sprintf("… %d more evidence rows in JSON", len(finding.Evidence)-i)))
						break
					}
					fmt.Fprintf(out, "    %s\n", style.dim("• "+evidence))
				}
				if finding.Remediation != "" {
					fmt.Fprintf(out, "    %s\n", style.dim("→ "+finding.Remediation))
				}
			}
		}
		fmt.Fprintln(out)
	}
	if full && len(suppressed) > 0 {
		fmt.Fprintf(out, "%s\n", style.dim(fmt.Sprintf("SUPPRESSED · %d", len(suppressed))))
		for _, finding := range suppressed {
			fmt.Fprintf(out, "  %s\n", style.dim(finding.Title+" — "+finding.SuppressionReason))
		}
		fmt.Fprintln(out)
	}
}

func renderHealth(out *strings.Builder, style styler, health *oracle.Health) {
	if health == nil || !section(out, style, "health", health.Section) {
		return
	}
	fmt.Fprintf(out, "  transactions %.2f/s · executions %.2f/s · parses %.2f/s · hard parses %.1f%%\n",
		health.TransactionsPerSec, health.ExecutionsPerSec, health.ParsesPerSec, health.HardParseRatio*100)
	fmt.Fprintf(out, "  logical reads %.2f/s · physical reads %.2f/s · redo %s/s\n\n",
		health.LogicalReadsPerS, health.PhysicalReadsPerS, humanBytes(int64(health.RedoBytesPerSec)))
}

func renderSessions(out *strings.Builder, style styler, sessions *oracle.Sessions) {
	if sessions == nil || !section(out, style, "sessions", sessions.Section) {
		return
	}
	fmt.Fprintf(out, "  %d total · %d active · %d inactive · %d blocked · %d active over five minutes\n\n",
		sessions.Total, sessions.Active, sessions.Inactive, sessions.Blocked, sessions.LongRunning)
}

func renderStorage(out *strings.Builder, style styler, storage *oracle.Storage) {
	if storage == nil || !section(out, style, "storage", storage.Section) {
		return
	}
	if len(storage.Tablespaces) == 0 {
		fmt.Fprintln(out, "  no permanent tablespaces visible")
	} else {
		for _, tablespace := range storage.Tablespaces {
			fmt.Fprintf(out, "  %-24s %6.1f%% allocated used · %s / %s · max %s\n",
				tablespace.Name, tablespace.UsedRatio*100, humanBytes(tablespace.UsedBytes),
				humanBytes(tablespace.TotalBytes), humanBytes(tablespace.MaxBytes))
		}
	}
	fmt.Fprintln(out)
}

func renderResources(out *strings.Builder, style styler, resources *oracle.Resources) {
	if resources == nil || !section(out, style, "resource limits", resources.Section) {
		return
	}
	for _, resource := range resources.Limits {
		limit := "unlimited"
		if !resource.Unlimited {
			limit = fmt.Sprintf("%d", resource.Limit)
		}
		fmt.Fprintf(out, "  %-12s current %d · high water %d · limit %s\n",
			resource.Name, resource.CurrentUtilization, resource.MaxUtilization, limit)
	}
	fmt.Fprintln(out)
}

func renderRecovery(out *strings.Builder, style styler, recovery *oracle.Recovery) {
	if recovery == nil || !section(out, style, "recovery", recovery.Section) {
		return
	}
	fmt.Fprintf(out, "  %s · %s · protection %s / %s · switchover %s\n",
		recovery.DatabaseRole, recovery.OpenMode, recovery.ProtectionMode, recovery.ProtectionLevel, recovery.SwitchoverStatus)
	for _, destination := range recovery.ArchiveDestinations {
		errorText := ""
		if destination.Error != "" {
			errorText = " · " + destination.Error
		}
		fmt.Fprintf(out, "  archive %-20s %-10s target %s%s\n",
			destination.Name, destination.Status, destination.Target, errorText)
	}
	keys := sortedKeys(recovery.DataGuardStats)
	for _, name := range keys {
		fmt.Fprintf(out, "  Data Guard %-14s %s\n", name, recovery.DataGuardStats[name])
	}
	fmt.Fprintln(out)
}

func renderLocks(out *strings.Builder, style styler, locks *oracle.Locks) {
	if locks == nil || !section(out, style, "locks", locks.Section) {
		return
	}
	if len(locks.Blocked) == 0 {
		fmt.Fprintln(out, "  no blocked sessions")
	} else {
		for _, row := range locks.Blocked {
			fmt.Fprintf(out, "  %d:%d blocked by %d:%d · %.0fs · %s · %s\n",
				row.Instance, row.SID, row.BlockingInstance, row.BlockingSession,
				row.SecondsInWait, row.WaitClass, row.Event)
		}
	}
	fmt.Fprintln(out)
}

func renderTopSQL(out *strings.Builder, style styler, topSQL *oracle.TopSQL) {
	if topSQL == nil || !section(out, style, "top SQL", topSQL.Section) {
		return
	}
	if len(topSQL.Statements) == 0 {
		fmt.Fprintln(out, "  no SQL statements visible")
	} else {
		for _, statement := range topSQL.Statements {
			fmt.Fprintf(out, "  %s · %.2fs elapsed · %.2fs CPU · %d executions · %s\n",
				statement.SQLID, statement.ElapsedSeconds, statement.CPUSeconds,
				statement.Executions, truncate(statement.SampleText, 90))
		}
	}
	fmt.Fprintln(out)
}

func renderMemory(out *strings.Builder, style styler, memory *oracle.Memory) {
	if memory == nil || !section(out, style, "memory", memory.Section) {
		return
	}
	fmt.Fprintf(out, "  SGA %s · PGA allocated %s\n\n", humanBytes(memory.SGABytes), humanBytes(memory.PGABytes))
}

func renderTables(out *strings.Builder, style styler, tables *oracle.Tables) {
	if tables == nil || !section(out, style, "tables", tables.Section) {
		return
	}
	stale, never := 0, 0
	for _, table := range tables.Rows {
		if table.StaleStats {
			stale++
		}
		if table.LastAnalyzed == nil && table.Rows > 0 {
			never++
		}
	}
	fmt.Fprintf(out, "  %d visible · %d stale · %d non-empty never analyzed%s\n\n",
		len(tables.Rows), stale, never, truncatedMark(tables.Truncated))
}

func renderIndexes(out *strings.Builder, style styler, indexes *oracle.Indexes) {
	if indexes == nil || !section(out, style, "indexes", indexes.Section) {
		return
	}
	unusable, invisible := 0, 0
	for _, index := range indexes.Rows {
		if strings.EqualFold(index.Status, "UNUSABLE") {
			unusable++
		}
		if strings.EqualFold(index.Visibility, "INVISIBLE") {
			invisible++
		}
	}
	fmt.Fprintf(out, "  %d visible to role · %d unusable · %d invisible%s\n\n",
		len(indexes.Rows), unusable, invisible, truncatedMark(indexes.Truncated))
}

func renderConfiguration(out *strings.Builder, style styler, configuration *oracle.Configuration) {
	if configuration == nil || !section(out, style, "configuration", configuration.Section) {
		return
	}
	keys := sortedKeys(configuration.Parameters)
	for _, name := range keys {
		parameter := configuration.Parameters[name]
		marker := ""
		if !parameter.Default {
			marker = " · " + parameter.ModifiedBy
		}
		fmt.Fprintf(out, "  %-28s %s%s\n", name, parameter.Value, marker)
	}
	fmt.Fprintln(out)
}

func section(out *strings.Builder, style styler, name string, section engine.Section) bool {
	fmt.Fprintf(out, "%s  %s\n", style.head(strings.ToUpper(name)), style.dim(section.Exactness))
	if section.Exactness == model.ExactnessUnavailable {
		fmt.Fprintf(out, "  %s\n\n", style.dim(section.Reason))
		return false
	}
	if section.Exactness == model.ExactnessReset {
		fmt.Fprintf(out, "  %s\n\n", style.warn(section.Reason))
		return false
	}
	return true
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
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
