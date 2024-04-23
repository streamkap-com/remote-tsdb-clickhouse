package clickhouse

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/prometheus/prometheus/prompb"
)

const minStepHintMs = 2000 // only consider sampling data when step is larger than this

func (ch *ClickHouseAdapter) ReadRequest(ctx context.Context, req *prompb.ReadRequest) (*prompb.ReadResponse, error) {
	res := &prompb.ReadResponse{}

	for _, q := range req.Queries {
		qresults := &prompb.QueryResult{}
		res.Results = append(res.Results, qresults)

		sb := &sqlBuilder{}

		sb.Clause("t >= ?", q.StartTimestampMs/1000)

		if q.EndTimestampMs > 0 {
			sb.Clause("t <= ?", q.EndTimestampMs/1000)
		}

		if err := addMatcherClauses(q.Matchers, sb, ch.readIgnoreLabel); err != nil {
			return nil, err
		}

		timeField := "updated_at"

		// When plotting graphs, Prometheus or Grafana may suggest returning
		// fewer than all datapoints by hinting with "StepMs" and "RangeMs".
		if q.Hints.StepMs > minStepHintMs && ch.readIgnoreHints == false {
			interval := q.Hints.StepMs
			if q.Hints.RangeMs > 0 && q.Hints.RangeMs < q.Hints.StepMs {
				interval = q.Hints.RangeMs
			}

			// The hints seem optimistic, return more datapoints than asked for.
			interval /= 2

			// DateTime field requires seconds
			interval /= 1000

			if interval < 1 {
				interval = 1
			}

			timeField = fmt.Sprintf("toStartOfInterval(updated_at, INTERVAL %d second)", interval)
		}

		rows, err := ch.db.QueryContext(ctx, "SELECT metric_name, arraySort(labels) as slb, "+timeField+" AS t, max(value) as max_0 FROM "+ch.table+" WHERE "+sb.Where()+" GROUP BY metric_name, slb, t ORDER BY metric_name, slb, t", sb.Args()...)
		if err != nil {
			return nil, err
		}

		// Fill out a single TimeSeries as long as metric_name and labels are the same.
		var lastName string
		var lastLabels []string
		var thisTimeseries *prompb.TimeSeries

		for rows.Next() {
			var name string
			var labels []string
			var updatedAt time.Time
			var value float64
			err := rows.Scan(&name, &labels, &updatedAt, &value)
			if err != nil {
				return nil, err
			}

			if thisTimeseries == nil || lastName != name || !slices.Equal(lastLabels, labels) {
				lastName = name
				lastLabels = labels

				thisTimeseries = &prompb.TimeSeries{}
				qresults.Timeseries = append(qresults.Timeseries, thisTimeseries)

				promlabs := []prompb.Label{{Name: "__name__", Value: name}}
				for _, label := range labels {
					ln, lv, _ := strings.Cut(label, "=")
					promlabs = append(promlabs, prompb.Label{Name: ln, Value: lv})
				}
				thisTimeseries.Labels = promlabs
			}

			thisTimeseries.Samples = append(thisTimeseries.Samples, prompb.Sample{Value: value, Timestamp: updatedAt.UnixMilli()})
		}

		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	return res, nil
}

func addMatcherClauses(matchers []*prompb.LabelMatcher, sb *sqlBuilder, readIgnoreLabel string) error {
	// NOTE: The match() calls use concat() to anchor the regexes to match prometheus behavior.
	for _, m := range matchers {
		if m.Name == "__name__" {
			switch m.Type {
			case prompb.LabelMatcher_EQ:
				sb.Clause("metric_name=?", m.Value)
			case prompb.LabelMatcher_NEQ:
				sb.Clause("metric_name!=?", m.Value) // Don't do this.
			case prompb.LabelMatcher_RE:
				sb.Clause("match(metric_name, concat(?, ?, ?))", "^", m.Value, "$")
			case prompb.LabelMatcher_NRE:
				sb.Clause("NOT match(metric_name, concat(?, ?, ?))", "^", m.Value, "$") // Don't do this.
			default:
				return fmt.Errorf("unsupported LabelMatcher_Type %v", m.Type)
			}
		} else {
			label := m.Name + "=" + m.Value
			switch m.Type {
			case prompb.LabelMatcher_EQ:
				if label == readIgnoreLabel {
					continue
				}
				sb.Clause("has(labels, ?)", label)
			case prompb.LabelMatcher_NEQ:
				sb.Clause("NOT has(labels, ?)", label)
			case prompb.LabelMatcher_RE:
				sb.Clause("arrayExists(x -> match(x, concat(?, ?, ?)), labels)", "^", label, "$")
			case prompb.LabelMatcher_NRE:
				sb.Clause("NOT arrayExists(x -> match(x, concat(?, ?, ?)), labels)", "^", label, "$")
			default:
				return fmt.Errorf("unsupported LabelMatcher_Type %v", m.Type)
			}
		}
	}
	return nil
}
