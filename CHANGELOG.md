# Changelog

All notable changes to this Streamkap fork of `remote-tsdb-clickhouse` are
documented here. Format follows [Keep a Changelog](https://keepachangelog.com/);
image tags are `vMAJOR.MINOR.PATCH` and map 1:1 to ECR tags at
`003675007768.dkr.ecr.us-west-2.amazonaws.com/remote-tsdb-clickhouse`.

## [v0.1.4] - 2026-07-22

### Changed
- **Write path now drops ALL NaN samples**, not just Prometheus staleness
  markers. `internal/clickhouse/write.go` swaps `value.IsStaleNaN(s.Value)` for
  `math.IsNaN(s.Value)`. v0.1.3 caught only ~0.7% of NaN rows (the staleness
  sentinel); the remaining 99.3% are generic exporter `math.NaN()`, and for the
  OpenMetrics stateset metric `kafka_connect_connector_status` those are the
  "inactive state" mirror rows (`status=running,value=NaN` while failed, etc.)
  that collapse into the same `anyLast` bucket and poison
  `all_time_anyLast_metrics` - the root of the stale connector_status false
  positives (findigs 2026-07-22) and false negatives (2026-05-08 silent
  failures).
- Renamed metric `samples_dropped_stale_total` -> `samples_dropped_nan_total`
  (now counts staleness markers + generic NaN). **Dashboards/alerts referencing
  the old name must be updated.**
- `internal/clickhouse/write_test.go`: contract flipped - generic `math.NaN()`
  is now expected to be dropped (was preserved in v0.1.3).

### Notes
- Does NOT address the leaked-MBean stale series (a frozen `value=1 failed`);
  that remains owned by the KC-REST cross-check in `trigger_runner.py`. See
  `docs/sre/runbook/runbook-connector-status-alert-stale-task-metric.md`.
- Rollout is a staged canary (us-west-2 first). See
  `docs/tsdb-clickhouse/2026-07-22-v0.1.4-drop-all-nan.md`.

## [v0.1.3] - 2026-05-24

### Added
- Write-path filter dropping Prometheus **staleness-marker** NaN
  (`0x7ff0000000000002`); new counter `samples_dropped_stale_total`.
- `.github/workflows/release.yml` - manual `workflow_dispatch` that validates a
  semver tag, assumes `StreamkapCIRole` via OIDC, and builds/pushes the image to
  ECR.

### Changed
- Dockerfile + `go.mod` builder bumped Go 1.20 -> 1.26 (required by the
  `prometheus/prometheus` import pulled in for `value.IsStaleNaN`).

### Deployment
- Canaried to `aws/prod/us-west-2` only. Verification found the narrow filter
  technically works but catches only ~0.7% of NaN; broadening recommended (-> v0.1.4).
  See `docs/tsdb-clickhouse/2026-05-24-v0.1.3-staleness-marker-filter.md`.

## [v0.1.2] - base

- Fork baseline of `jamessanford/remote-tsdb-clickhouse`: Prometheus
  `remote_write` receiver that forwards every sample as one row into a ClickHouse
  table (`metrics_distributed`, `metrics_flink_distributed`,
  `metrics_kafka_distributed`, `metrics_system_distributed`). Wrote all samples
  indiscriminately, including NaN (stored as NULL).

[v0.1.4]: https://github.com/streamkap-com/remote-tsdb-clickhouse/compare/v0.1.3...v0.1.4
[v0.1.3]: https://github.com/streamkap-com/remote-tsdb-clickhouse/compare/v0.1.2...v0.1.3
