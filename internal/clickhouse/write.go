package clickhouse

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/prometheus/model/value"
	"github.com/prometheus/prometheus/prompb"
)

// WriteRequest writes Prometheus remote_write samples into ClickHouse.
// Returns (written, droppedStale, err). droppedStale counts Prometheus
// staleness markers (NaN with the bit pattern 0x7ff0000000000002) that
// were skipped because they signal "this series ended", not real data.
// Letting them through produces NULL rows in metrics_* that poison
// anyLast() aggregations in the all_time_* materialized views.
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
	droppedStale := 0

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
			if value.IsStaleNaN(s.Value) {
				droppedStale++
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
	return written, droppedStale, err
}
