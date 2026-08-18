// Package findings computes deterministic Oracle TimesTen findings.
package findings

import (
	"fmt"
	"sort"

	"github.com/pgrundev/pgbot/internal/engine/timesten"
	"github.com/pgrundev/pgbot/internal/model"
)

const (
	spaceWarn          = 0.85
	spaceCritical      = 0.95
	maxFindingEvidence = 50
)

// Compute returns TimesTen findings in deterministic priority order.
func Compute(data *timesten.Data) []model.Finding {
	if data == nil {
		return []model.Finding{}
	}
	var findings []model.Finding
	findings = append(findings, spacePressure(data)...)
	findings = append(findings, recoveryRequired(data)...)
	findings = append(findings, lockEvents(data)...)
	findings = append(findings, logBufferWaits(data)...)
	findings = append(findings, missingStatistics(data)...)

	sort.SliceStable(findings, func(i, j int) bool {
		left, right := severityRank(findings[i].Severity), severityRank(findings[j].Severity)
		if left != right {
			return left > right
		}
		if findings[i].Impact.Score != findings[j].Impact.Score {
			return findings[i].Impact.Score > findings[j].Impact.Score
		}
		if findings[i].ID != findings[j].ID {
			return findings[i].ID < findings[j].ID
		}
		return findings[i].Object < findings[j].Object
	})
	if findings == nil {
		return []model.Finding{}
	}
	return findings
}

func spacePressure(data *timesten.Data) []model.Finding {
	if data.Space == nil || data.Space.Exactness == model.ExactnessUnavailable {
		return nil
	}
	pools := []struct {
		name string
		pool timesten.SpacePool
	}{
		{"permanent", data.Space.Permanent},
		{"temporary", data.Space.Temporary},
	}
	var findings []model.Finding
	for _, item := range pools {
		if item.pool.AllocatedBytes <= 0 || item.pool.UsedRatio < spaceWarn {
			continue
		}
		severity := model.SeverityWarn
		score := float64(72)
		if item.pool.UsedRatio >= spaceCritical {
			severity = model.SeverityCritical
			score = 94
		}
		findings = append(findings, model.Finding{
			ID:       "timesten_space_pressure",
			Object:   "space:" + item.name,
			Severity: severity,
			Title:    fmt.Sprintf("TimesTen %s space is %.0f%% used", item.name, item.pool.UsedRatio*100),
			Detail:   "Current in-memory allocation is close to the configured TimesTen space limit.",
			Evidence: []string{fmt.Sprintf(
				"pool=%s used=%d allocated=%d high_water=%d ratio=%.4f",
				item.name, item.pool.UsedBytes, item.pool.AllocatedBytes, item.pool.HighWaterBytes, item.pool.UsedRatio,
			)},
			Remediation: "Confirm workload growth and table aging behavior. Increase the TimesTen space setting only after you verify host memory and restart requirements.",
			Impact: model.Impact{
				Score:     score,
				Dimension: model.DimRisk,
				Estimate:  fmt.Sprintf("%.0f%% of %s", item.pool.UsedRatio*100, humanBytes(item.pool.AllocatedBytes)),
				Basis:     "SYS.MONITOR current in-use size divided by allocated size",
			},
			Confidence: 0.98,
		})
	}
	return findings
}

func recoveryRequired(data *timesten.Data) []model.Finding {
	if data.Persistence == nil || data.Persistence.Exactness == model.ExactnessUnavailable || !data.Persistence.RequiredRecovery {
		return nil
	}
	return []model.Finding{{
		ID:          "timesten_recovery_required",
		Severity:    model.SeverityCritical,
		Title:       "TimesTen reports required recovery",
		Detail:      "SYS.MONITOR reports that the database requires recovery work before normal operation is safe.",
		Evidence:    []string{fmt.Sprintf("required_recovery=true first_log=%d last_log=%d", data.Persistence.FirstLogFile, data.Persistence.LastLogFile)},
		Remediation: "Inspect the TimesTen daemon and database logs, confirm the recovery state with the database administrator, and follow the documented TimesTen recovery procedure.",
		Impact: model.Impact{
			Score:     100,
			Dimension: model.DimRisk,
			Estimate:  "database recovery flag is set",
			Basis:     "SYS.MONITOR REQUIRED_RECOVERY",
		},
		Confidence:    1,
		ClusterScoped: true,
	}}
}

func lockEvents(data *timesten.Data) []model.Finding {
	if data.Locks == nil || data.Locks.Exactness == model.ExactnessUnavailable || data.Locks.Exactness == model.ExactnessReset {
		return nil
	}
	var findings []model.Finding
	if data.Locks.DeadlocksPerSec > 0 {
		findings = append(findings, model.Finding{
			ID:            "timesten_deadlocks",
			Severity:      model.SeverityCritical,
			Title:         "TimesTen deadlocks occurred during the sample",
			Detail:        "The deadlock counter increased between the two report samples.",
			Evidence:      []string{fmt.Sprintf("deadlocks_per_sec=%.2f cumulative=%d", data.Locks.DeadlocksPerSec, data.Locks.Deadlocks)},
			Remediation:   "Use ttXactAdmin outside ttbot to identify the involved transactions, then correct transaction order and scope in the application.",
			Impact:        model.Impact{Score: 96, Dimension: model.DimRisk, Estimate: fmt.Sprintf("%.2f deadlocks/s", data.Locks.DeadlocksPerSec), Basis: "sampled SYS.SYSTEMSTATS lock.deadlocks delta"},
			Confidence:    0.99,
			ClusterScoped: true,
		})
	}
	if data.Locks.TimeoutsPerSec > 0 || data.Locks.WaitGrantsPerSec > 0 {
		findings = append(findings, model.Finding{
			ID:            "timesten_lock_contention",
			Severity:      model.SeverityWarn,
			Title:         "TimesTen lock contention occurred during the sample",
			Detail:        "Lock timeouts or lock grants after waiting increased during the report window.",
			Evidence:      []string{fmt.Sprintf("timeouts_per_sec=%.2f waits_per_sec=%.2f cumulative_timeouts=%d cumulative_waits=%d", data.Locks.TimeoutsPerSec, data.Locks.WaitGrantsPerSec, data.Locks.Timeouts, data.Locks.WaitGrants)},
			Remediation:   "Inspect active transactions with ttXactAdmin outside ttbot. Keep transactions short and use a consistent object access order.",
			Impact:        model.Impact{Score: 70, Dimension: model.DimLatency, Estimate: fmt.Sprintf("%.2f waits/s", data.Locks.WaitGrantsPerSec), Basis: "sampled SYS.SYSTEMSTATS lock timeout and wait-grant deltas"},
			Confidence:    0.9,
			ClusterScoped: true,
		})
	}
	return findings
}

func logBufferWaits(data *timesten.Data) []model.Finding {
	if data.Health == nil || data.Health.Exactness == model.ExactnessUnavailable || data.Health.Exactness == model.ExactnessReset || data.Health.LogBufferWaitsPerS <= 0 {
		return nil
	}
	return []model.Finding{{
		ID:            "timesten_log_buffer_waits",
		Severity:      model.SeverityWarn,
		Title:         "TimesTen log buffer waits occurred during the sample",
		Detail:        "Transactions waited for free space in the in-memory log buffer during the report window.",
		Evidence:      []string{fmt.Sprintf("log_buffer_waits_per_sec=%.2f log_bytes_per_sec=%.2f", data.Health.LogBufferWaitsPerS, data.Health.LogBytesPerSec)},
		Remediation:   "Review transaction size, commit rate, LogBufMB, and log flush throughput. Change LogBufMB only after you verify memory capacity and workload behavior.",
		Impact:        model.Impact{Score: 66, Dimension: model.DimLatency, Estimate: fmt.Sprintf("%.2f waits/s", data.Health.LogBufferWaitsPerS), Basis: "sampled SYS.SYSTEMSTATS log.buffer.waits delta"},
		Confidence:    0.95,
		ClusterScoped: true,
	}}
}

func missingStatistics(data *timesten.Data) []model.Finding {
	if data.Tables == nil || data.Tables.Exactness == model.ExactnessUnavailable {
		return nil
	}
	var evidence, objects []string
	total := 0
	for _, table := range data.Tables.Rows {
		if !table.MissingStatistics {
			continue
		}
		total++
		if len(evidence) < maxFindingEvidence {
			object := table.Owner + "." + table.Name
			evidence = append(evidence, fmt.Sprintf("table=%s estimated_rows=%d last_stats_update=missing", object, table.EstimatedRows))
			objects = append(objects, object)
		}
	}
	if total == 0 {
		return nil
	}
	verb := "lack"
	if total == 1 {
		verb = "lacks"
	}
	caveats := []string{"SYS.TBL_STATS reports stored optimizer statistics, not live row counts."}
	if data.Tables.Truncated {
		caveats = append(caveats, "The table inventory is limited to 500 rows, so this finding can be incomplete.")
	}
	return []model.Finding{{
		ID:          "timesten_missing_table_statistics",
		Severity:    model.SeverityWarn,
		Title:       fmt.Sprintf("%d TimesTen table%s %s optimizer statistics", total, plural(total), verb),
		Detail:      "Non-empty application tables have no recorded optimizer statistics update.",
		Evidence:    evidence,
		Objects:     objects,
		Remediation: "Confirm the table workload, then update optimizer statistics during a suitable window with the documented TimesTen statistics procedure.",
		Impact:      model.Impact{Score: 52, Dimension: model.DimThroughput, Estimate: fmt.Sprintf("at least %d table%s", total, plural(total)), Basis: "SYS.TABLES row estimate joined to SYS.TBL_STATS"},
		Confidence:  0.8,
		Caveats:     caveats,
	}}
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

func plural(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
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
