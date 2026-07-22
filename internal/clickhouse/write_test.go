package clickhouse

import (
	"math"
	"testing"

	"github.com/prometheus/prometheus/model/value"
)

// Documents the contract the write path depends on (v0.1.4): ALL NaN is
// dropped - both the Prometheus staleness-marker bit pattern AND a generic
// math.NaN() from an exporter. A Float64 NaN stores as NULL in ClickHouse and
// is operationally useless; for the stateset metric kafka_connect_connector_status
// the generic-NaN "inactive state" mirror rows are what poison anyLast in
// all_time_anyLast_metrics (see docs/sre/alerts-report/2026-05-08-skap-alerts-silent-failures.md).
// This flips the deliberately-narrow v0.1.3 contract, which preserved generic NaN.
func TestNaNDetection(t *testing.T) {
	staleSample := math.Float64frombits(value.StaleNaN)

	if !math.IsNaN(staleSample) {
		t.Fatalf("Prometheus StaleNaN bit pattern (%#x) must be dropped as NaN", value.StaleNaN)
	}

	if !math.IsNaN(math.NaN()) {
		t.Fatalf("generic math.NaN() must be dropped as NaN (v0.1.4 contract flip)")
	}

	if math.IsNaN(1.0) {
		t.Fatalf("ordinary values must not be treated as NaN")
	}
	if math.IsNaN(0.0) {
		t.Fatalf("zero must not be treated as NaN")
	}
}
