package clickhouse

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/prometheus/prometheus/prompb"
)

// WriteRequest writes Prometheus remote_write samples into ClickHouse.
// Returns (written, droppedNaN, err). droppedNaN counts ALL NaN samples -
// both Prometheus staleness markers (bit pattern 0x7ff0000000000002) and
// generic math.NaN() emitted by exporters as "no value right now". Both
// serialize as NULL in the metrics_* Float64 column and are operationally
// useless; more importantly, for the OpenMetrics stateset metric
// kafka_connect_connector_status the generic-NaN "inactive state" mirror
// rows collapse into the same anyLast bucket (status is not in the MV key)
// and poison anyLast() in the all_time_* materialized views, producing the
// stale connector_status false positives/negatives. See
// docs/sre/alerts-report/2026-05-08-skap-alerts-silent-failures.md.
func (ch *ClickHouseAdapter) WriteRequest(ctx context.Context, req *prompb.WriteRequest) (int, int, error) {
	commitDone := false

	tx, err := ch.db.Begin()
	if err != nil {
		return 0, 0, err
	}
	defer func() {
		if !commitDone {
			tx.Rollback()
		}
	}()

	// NOTE: Value of ch.table is sanitized in NewClickHouseAdapter.
	stmt, err := tx.PrepareContext(ctx, fmt.Sprintf("INSERT INTO %s (updated_at, metric_name, labels, value)", ch.table))
	if err != nil {
		return 0, 0, err
	}
	defer stmt.Close()

	written := 0
	droppedNaN := 0

	for _, t := range req.Timeseries {
		var name string
		labels := make([]string, 0, len(t.Labels))

		for _, l := range t.Labels {
			if l.Name == "__name__" {
				name = l.Value
				continue
			}
			labels = append(labels, l.Name+"="+l.Value)
		}

		for _, s := range t.Samples {
			if math.IsNaN(s.Value) {
				droppedNaN++
				continue
			}
			_, err = stmt.Exec(
				time.UnixMilli(s.Timestamp).UTC(), // updated_at
				name,                              // metric_name
				labels,                            // labels
				s.Value,                           // value
			)
			if err != nil {
				return 0, 0, err
			}
			written++
		}
	}

	err = tx.Commit()
	commitDone = true
	return written, droppedNaN, err
}
