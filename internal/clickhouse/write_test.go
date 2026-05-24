package clickhouse

import (
	"math"
	"testing"

	"github.com/prometheus/prometheus/model/value"
)

// Documents the contract we depend on: only Prometheus's staleness-marker NaN
// is treated as stale. A regular math.NaN() (e.g. division-by-zero from an
// exporter) is NOT stale and should still be written through so that broken
// exporters remain visible in ClickHouse instead of being silently swallowed.
func TestStaleNaNDetection(t *testing.T) {
	staleSample := math.Float64frombits(value.StaleNaN)

	if !value.IsStaleNaN(staleSample) {
		t.Fatalf("Prometheus StaleNaN bit pattern (%#x) must be detected as a staleness marker", value.StaleNaN)
	}

	if value.IsStaleNaN(math.NaN()) {
		t.Fatalf("generic math.NaN() must NOT be treated as a staleness marker; " +
			"only the Prometheus-specific bit pattern is the stale sentinel")
	}

	if value.IsStaleNaN(1.0) {
		t.Fatalf("ordinary values must not be treated as stale")
	}
	if value.IsStaleNaN(0.0) {
		t.Fatalf("zero must not be treated as stale")
	}
}
