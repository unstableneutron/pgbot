// Package findings computes deterministic Oracle Database findings.
package findings

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/pgrundev/pgbot/internal/engine/oracle"
	"github.com/pgrundev/pgbot/internal/model"
)

const (
	pressureWarn       = 0.85
	pressureCritical   = 0.95
	hardParseWarn      = 0.10
	hardParseCritical  = 0.30
	dataGuardLagWarn   = 60.0
	dataGuardLagCrit   = 300.0
	maxFindingEvidence = 50
)

var intervalValue = regexp.MustCompile(`^([+-]?)(\d+)\s+(\d+):(\d+):(\d+(?:\.\d+)?)`)

// Compute returns Oracle findings in deterministic priority order.
func Compute(data *oracle.Data) []model.Finding {
	if data == nil {
		return []model.Finding{}
	}
	var out []model.Finding
	out = append(out, blockingSessions(data)...)
	out = append(out, longRunningCalls(data)...)
	out = append(out, resourcePressure(data)...)
	out = append(out, tablespacePressure(data)...)
	out = append(out, hardParsing(data)...)
	out = append(out, staleStatistics(data)...)
	out = append(out, unusableIndexes(data)...)
	out = append(out, archiveFailures(data)...)
	out = append(out, dataGuardLag(data)...)
	out = append(out, configurationRisks(data)...)

	sort.SliceStable(out, func(i, j int) bool {
		left, right := severityRank(out[i].Severity), severityRank(out[j].Severity)
		if left != right {
			return left > right
		}
		if out[i].Impact.Score != out[j].Impact.Score {
			return out[i].Impact.Score > out[j].Impact.Score
		}
		if out[i].ID != out[j].ID {
			return out[i].ID < out[j].ID
		}
		return out[i].Object < out[j].Object
	})
	if out == nil {
		return []model.Finding{}
	}
	return out
}

func blockingSessions(data *oracle.Data) []model.Finding {
	if data.Locks == nil || data.Locks.Exactness == model.ExactnessUnavailable || len(data.Locks.Blocked) == 0 {
		return nil
	}
	severity := model.SeverityWarn
	maxWait := float64(0)
	evidence := make([]string, 0, len(data.Locks.Blocked))
	for _, row := range data.Locks.Blocked {
		maxWait = math.Max(maxWait, row.SecondsInWait)
		evidence = append(evidence, fmt.Sprintf(
			"instance=%d sid=%d blocker=%d:%d wait=%s seconds=%.0f",
			row.Instance, row.SID, row.BlockingInstance, row.BlockingSession, row.Event, row.SecondsInWait,
		))
	}
	if maxWait >= 300 {
		severity = model.SeverityCritical
	}
	count := len(data.Locks.Blocked)
	return []model.Finding{{
		ID:          "oracle_blocking_sessions",
		Severity:    severity,
		Title:       fmt.Sprintf("%d Oracle session%s blocked", count, plural(count)),
		Detail:      "Sessions are waiting for another session to release a resource. This is point-in-time evidence from GV$SESSION.",
		Evidence:    evidence,
		Remediation: "Find the final blocker, confirm its current work and transaction state, then resolve the application or transaction cause before you terminate a session.",
		Impact: model.Impact{
			Score:     scoreForSeverity(severity, 88, 70),
			Dimension: model.DimRisk,
			Estimate:  fmt.Sprintf("%d blocked session%s; longest wait %.0fs", count, plural(count), maxWait),
			Basis:     "current GV$SESSION blocking_session and seconds_in_wait values",
		},
		Confidence: 0.95,
		Caveats:    []string{"The session view is a point-in-time sample; confirm that the block still exists before intervention."},
	}}
}

func longRunningCalls(data *oracle.Data) []model.Finding {
	if data.Sessions == nil || data.Sessions.Exactness == model.ExactnessUnavailable || data.Sessions.LongRunning == 0 {
		return nil
	}
	count := data.Sessions.LongRunning
	return []model.Finding{{
		ID:          "oracle_long_running_calls",
		Severity:    model.SeverityWarn,
		Title:       fmt.Sprintf("%d active Oracle call%s over five minutes", count, plural64(count)),
		Detail:      "GV$SESSION reports active calls whose last_call_et is at least 300 seconds.",
		Remediation: "Review the SQL, wait event, plan, and business purpose of each long call before you cancel it.",
		Impact: model.Impact{
			Score:     55,
			Dimension: model.DimLatency,
			Estimate:  fmt.Sprintf("%d active call%s", count, plural64(count)),
			Basis:     "active non-background sessions with last_call_et >= 300",
		},
		Confidence: 0.65,
		Caveats:    []string{"A long active call can be expected batch work; duration alone does not show a defect."},
	}}
}

func resourcePressure(data *oracle.Data) []model.Finding {
	if data.Resources == nil || data.Resources.Exactness == model.ExactnessUnavailable {
		return nil
	}
	var out []model.Finding
	for _, resource := range data.Resources.Limits {
		if resource.Unlimited || resource.Limit <= 0 {
			continue
		}
		ratio := float64(resource.CurrentUtilization) / float64(resource.Limit)
		if ratio < pressureWarn {
			continue
		}
		severity := pressureSeverity(ratio)
		out = append(out, model.Finding{
			ID:       "oracle_resource_limit_pressure",
			Object:   "resource:" + resource.Name,
			Severity: severity,
			Title:    fmt.Sprintf("Oracle %s limit is %.0f%% used", resource.Name, ratio*100),
			Detail:   "Current utilization is close to the configured instance resource limit.",
			Evidence: []string{fmt.Sprintf(
				"resource=%s current=%d high_water=%d limit=%d",
				resource.Name, resource.CurrentUtilization, resource.MaxUtilization, resource.Limit,
			)},
			Remediation: "Confirm workload growth and connection pool behavior. Increase the limit only after you confirm host capacity and restart requirements.",
			Impact: model.Impact{
				Score:     scoreForSeverity(severity, 90, 68),
				Dimension: model.DimRisk,
				Estimate:  fmt.Sprintf("%d of %d %s", resource.CurrentUtilization, resource.Limit, resource.Name),
				Basis:     "V$RESOURCE_LIMIT current_utilization / limit_value",
			},
			Confidence: 0.95,
		})
	}
	return out
}

func tablespacePressure(data *oracle.Data) []model.Finding {
	if data.Storage == nil || data.Storage.Exactness == model.ExactnessUnavailable {
		return nil
	}
	var out []model.Finding
	for _, tablespace := range data.Storage.Tablespaces {
		capacity := tablespace.MaxBytes
		if capacity < tablespace.TotalBytes {
			capacity = tablespace.TotalBytes
		}
		if capacity <= 0 {
			continue
		}
		ratio := float64(tablespace.UsedBytes) / float64(capacity)
		if ratio < pressureWarn {
			continue
		}
		severity := pressureSeverity(ratio)
		free := capacity - tablespace.UsedBytes
		if free < 0 {
			free = 0
		}
		out = append(out, model.Finding{
			ID:       "oracle_tablespace_pressure",
			Object:   "tablespace:" + tablespace.Name,
			Severity: severity,
			Title:    fmt.Sprintf("Tablespace %s is %.0f%% used", tablespace.Name, ratio*100),
			Detail:   "Used bytes are close to the data files' configured maximum capacity.",
			Evidence: []string{fmt.Sprintf(
				"tablespace=%s used=%s allocated=%s maximum=%s",
				tablespace.Name, humanBytes(tablespace.UsedBytes), humanBytes(tablespace.TotalBytes), humanBytes(capacity),
			)},
			Remediation: "Confirm filesystem or ASM capacity, data-file autoextend settings, and growth rate before you add or resize a data file.",
			Impact: model.Impact{
				Score:     scoreForSeverity(severity, 92, 72),
				Dimension: model.DimStorage,
				Estimate:  fmt.Sprintf("%s free to configured maximum", humanBytes(free)),
				Basis:     "used bytes / sum of each data file's greater allocated or max bytes",
			},
			Confidence: 0.9,
			Caveats:    []string{"This check does not include temporary tablespaces or filesystem and ASM free space."},
		})
	}
	return out
}

func hardParsing(data *oracle.Data) []model.Finding {
	if data.Health == nil || data.Health.Exactness != model.ExactnessSampled ||
		data.Health.ParsesPerSec < 5 || data.Health.HardParseRatio < hardParseWarn {
		return nil
	}
	severity := model.SeverityWarn
	if data.Health.HardParseRatio >= hardParseCritical && data.Health.HardParsesPerSec >= 20 {
		severity = model.SeverityCritical
	}
	return []model.Finding{{
		ID:       "oracle_hard_parse_pressure",
		Severity: severity,
		Title:    fmt.Sprintf("%.0f%% of Oracle parses are hard parses", data.Health.HardParseRatio*100),
		Detail:   "A high hard-parse share can consume CPU and library-cache latches and can show poor cursor reuse.",
		Evidence: []string{fmt.Sprintf(
			"parses_per_sec=%.2f hard_parses_per_sec=%.2f ratio=%.3f",
			data.Health.ParsesPerSec, data.Health.HardParsesPerSec, data.Health.HardParseRatio,
		)},
		Remediation: "Check bind use, cursor sharing, invalidations, and application connection behavior. Do not change cursor_sharing without workload tests.",
		Impact: model.Impact{
			Score:     scoreForSeverity(severity, 80, 60),
			Dimension: model.DimThroughput,
			Estimate:  fmt.Sprintf("%.2f hard parses/s", data.Health.HardParsesPerSec),
			Basis:     "two-sample V$SYSSTAT hard parse count / total parse count",
		},
		Confidence: 0.8,
		Caveats:    []string{"A short sample can reflect a deployment or connection burst; confirm the rate across more than one run."},
	}}
}

func staleStatistics(data *oracle.Data) []model.Finding {
	if data.Tables == nil || data.Tables.Exactness == model.ExactnessUnavailable {
		return nil
	}
	var evidence, objects []string
	total := 0
	for _, table := range data.Tables.Rows {
		if table.Rows <= 0 || (!table.StaleStats && table.LastAnalyzed != nil) {
			continue
		}
		name := table.Owner + "." + table.Name
		total++
		state := "stale"
		if table.LastAnalyzed == nil {
			state = "never analyzed"
		}
		if len(evidence) < maxFindingEvidence {
			evidence = append(evidence, fmt.Sprintf("%s rows=%d statistics=%s", name, table.Rows, state))
			objects = append(objects, name)
		}
	}
	if len(evidence) == 0 {
		return nil
	}
	caveats := []string{"Stale statistics are an Oracle-maintained signal; confirm that the table has representative workload before an urgent gather."}
	if data.Tables.Truncated {
		caveats = append(caveats, "The table inventory is limited to 500 rows, so this finding can be incomplete.")
	}
	return []model.Finding{{
		ID:          "oracle_stale_statistics",
		Severity:    model.SeverityWarn,
		Title:       fmt.Sprintf("%d Oracle table%s need statistics review", total, plural(total)),
		Detail:      "Non-empty tables report stale statistics or no last analysis time.",
		Evidence:    evidence,
		Objects:     objects,
		Remediation: "Confirm automatic optimizer statistics collection, then gather representative statistics for affected schemas or tables during an appropriate window.",
		Impact: model.Impact{
			Score:     58,
			Dimension: model.DimThroughput,
			Estimate:  fmt.Sprintf("at least %d table%s", total, plural(total)),
			Basis:     "DBA_TAB_STATISTICS stale_stats and last_analyzed",
		},
		Confidence: 0.8,
		Caveats:    caveats,
	}}
}

func unusableIndexes(data *oracle.Data) []model.Finding {
	if data.Indexes == nil || data.Indexes.Exactness == model.ExactnessUnavailable {
		return nil
	}
	var evidence, objects []string
	total := 0
	for _, index := range data.Indexes.Rows {
		if !strings.EqualFold(index.Status, "UNUSABLE") {
			continue
		}
		name := index.Owner + "." + index.Name
		total++
		if len(evidence) < maxFindingEvidence {
			evidence = append(evidence, fmt.Sprintf("%s on %s.%s status=%s", name, index.TableOwner, index.TableName, index.Status))
			objects = append(objects, name)
		}
	}
	if len(evidence) == 0 {
		return nil
	}
	caveats := []string{"Partition-level index status is not part of this first collector; inspect index partitions before you choose a rebuild scope."}
	if data.Indexes.Truncated {
		caveats = append(caveats, "The index inventory is limited to 500 rows, so this finding can be incomplete.")
	}
	return []model.Finding{{
		ID:          "oracle_unusable_indexes",
		Severity:    model.SeverityCritical,
		Title:       fmt.Sprintf("%d Oracle index%s unusable", total, plural(total)),
		Detail:      "An unusable index cannot support normal access and can make DML fail when Oracle must maintain it.",
		Evidence:    evidence,
		Objects:     objects,
		Remediation: "Identify why each index became unusable, confirm partition scope, and rebuild or replace it with a tested change plan.",
		Impact: model.Impact{
			Score:     95,
			Dimension: model.DimRisk,
			Estimate:  fmt.Sprintf("at least %d unusable index%s", total, plural(total)),
			Basis:     "DBA_INDEXES status = UNUSABLE",
		},
		Confidence: 0.98,
		Caveats:    caveats,
	}}
}

func archiveFailures(data *oracle.Data) []model.Finding {
	if data.Recovery == nil || data.Recovery.Exactness == model.ExactnessUnavailable {
		return nil
	}
	var out []model.Finding
	for _, destination := range data.Recovery.ArchiveDestinations {
		status := strings.ToUpper(destination.Status)
		if destination.Error == "" && status != "ERROR" && status != "DISABLED" && status != "FULL" && status != "BAD PARAM" {
			continue
		}
		name := destination.Name
		if name == "" {
			name = fmt.Sprintf("destination %d", destination.ID)
		}
		out = append(out, model.Finding{
			ID:       "oracle_archive_destination_failure",
			Object:   "archive_destination:" + name,
			Severity: model.SeverityCritical,
			Title:    fmt.Sprintf("Oracle archive destination %s is %s", name, destination.Status),
			Detail:   "An enabled archive destination reports a failure state or error text.",
			Evidence: []string{fmt.Sprintf(
				"destination=%s target=%s status=%s error=%s",
				name, destination.Target, destination.Status, destination.Error,
			)},
			Remediation: "Check the destination error, storage or network reachability, and redo transport configuration. Restore archiving before redo storage fills.",
			Impact: model.Impact{
				Score:     98,
				Dimension: model.DimRisk,
				Estimate:  "archive or redo transport is impaired",
				Basis:     "V$ARCHIVE_DEST status and error",
			},
			Confidence: 0.98,
		})
	}
	return out
}

func dataGuardLag(data *oracle.Data) []model.Finding {
	if data.Recovery == nil || data.Recovery.Exactness == model.ExactnessUnavailable {
		return nil
	}
	var out []model.Finding
	for _, name := range []string{"transport lag", "apply lag"} {
		raw, ok := data.Recovery.DataGuardStats[name]
		if !ok {
			continue
		}
		seconds, ok := parseIntervalSeconds(raw)
		if !ok || seconds < dataGuardLagWarn {
			continue
		}
		severity := model.SeverityWarn
		if seconds >= dataGuardLagCrit {
			severity = model.SeverityCritical
		}
		out = append(out, model.Finding{
			ID:          "oracle_dataguard_lag",
			Object:      "dataguard:" + strings.ReplaceAll(name, " ", "_"),
			Severity:    severity,
			Title:       fmt.Sprintf("Oracle Data Guard %s is %s", name, humanDuration(seconds)),
			Detail:      "The target standby reports a redo transport or apply delay.",
			Evidence:    []string{fmt.Sprintf("metric=%s value=%s", name, raw)},
			Remediation: "Check redo generation, transport, standby receive and apply processes, network health, and archive gaps before failover is needed.",
			Impact: model.Impact{
				Score:     scoreForSeverity(severity, 94, 72),
				Dimension: model.DimRisk,
				Estimate:  fmt.Sprintf("%s %s", humanDuration(seconds), name),
				Basis:     "V$DATAGUARD_STATS interval value",
			},
			Confidence: 0.95,
		})
	}
	return out
}

func configurationRisks(data *oracle.Data) []model.Finding {
	if data.Configuration == nil || data.Configuration.Exactness == model.ExactnessUnavailable {
		return nil
	}
	parameter, ok := data.Configuration.Parameters["statistics_level"]
	if !ok || !strings.EqualFold(parameter.Value, "BASIC") {
		return nil
	}
	return []model.Finding{{
		ID:          "oracle_statistics_level_basic",
		Object:      "parameter:statistics_level",
		Severity:    model.SeverityWarn,
		Title:       "Oracle statistics_level is BASIC",
		Detail:      "BASIC disables collection of statistics that Oracle diagnostic and tuning features use.",
		Evidence:    []string{"statistics_level=BASIC"},
		Remediation: "Confirm why BASIC is required and test TYPICAL in the same workload and support policy before you change the parameter.",
		Impact: model.Impact{
			Score:     60,
			Dimension: model.DimThroughput,
			Estimate:  "reduced performance statistics and diagnostic visibility",
			Basis:     "V$PARAMETER display_value",
		},
		Confidence: 0.95,
	}}
}

func parseIntervalSeconds(value string) (float64, bool) {
	match := intervalValue.FindStringSubmatch(strings.TrimSpace(value))
	if match == nil {
		return 0, false
	}
	days, err1 := strconv.ParseFloat(match[2], 64)
	hours, err2 := strconv.ParseFloat(match[3], 64)
	minutes, err3 := strconv.ParseFloat(match[4], 64)
	seconds, err4 := strconv.ParseFloat(match[5], 64)
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		return 0, false
	}
	total := days*86400 + hours*3600 + minutes*60 + seconds
	if match[1] == "-" {
		total = -total
	}
	return total, true
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

func pressureSeverity(ratio float64) string {
	if ratio >= pressureCritical {
		return model.SeverityCritical
	}
	return model.SeverityWarn
}

func scoreForSeverity(severity string, critical, warning float64) float64 {
	if severity == model.SeverityCritical {
		return critical
	}
	return warning
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

func humanDuration(seconds float64) string {
	duration := int64(math.Round(seconds))
	switch {
	case duration >= 86400:
		return fmt.Sprintf("%dd%dh", duration/86400, (duration%86400)/3600)
	case duration >= 3600:
		return fmt.Sprintf("%dh%dm", duration/3600, (duration%3600)/60)
	case duration >= 60:
		return fmt.Sprintf("%dm%ds", duration/60, duration%60)
	default:
		return fmt.Sprintf("%ds", duration)
	}
}

func plural(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

func plural64(count int64) string {
	if count == 1 {
		return ""
	}
	return "s"
}
