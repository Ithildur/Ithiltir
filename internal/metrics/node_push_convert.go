package metrics

import (
	"time"

	"dash/internal/model"
)

// NormalizeReport rewrites ingest metadata to the server-side canonical form.
func NormalizeReport(serverID int64, displayOrder int, report NodeReport, receivedAt time.Time) (NodeReport, string) {
	report.ServerID = serverID
	report.DisplayOrder = displayOrder
	originalTimestamp := FormatTimestamp(report.Timestamp)
	report.SentAt = originalTimestamp
	report.Timestamp = receivedAt
	return report, originalTimestamp
}

// BuildMetric converts inbound report data into the history metric row and current runtime payload.
func BuildMetric(serverID int64, values Metrics, receivedAt time.Time, reportedAtRaw string) (model.ServerMetric, model.MetricRuntime, error) {
	snapshot, err := ToSnapshot(values)
	if err != nil {
		return model.ServerMetric{}, model.MetricRuntime{}, err
	}

	reportedAt := ParseReportedAt(reportedAtRaw)
	collectedAt := receivedAt.UTC()
	if reportedAt != nil {
		utc := reportedAt.UTC()
		reportedAt = &utc
	}

	return model.ServerMetric{
		ServerID:     serverID,
		CollectedAt:  collectedAt,
		ReportedAt:   reportedAt,
		MetricValues: snapshot.MetricValues,
	}, snapshot.MetricRuntime, nil
}
