# Status Algorithm

## Purpose

The algorithm decides whether a service is:

- `Operational`
- `Degraded`
- `Outage`

It combines two signal families:

- recent user outage reports
- recent probe failures

The implementation lives in `algorithm/status.go`. The same logic is reused by API responses and the incident worker through `utils/api-status.go` and `workers/incidents.go`.

## Inputs

The algorithm accepts `algorithm.Signals`:

- `RecentReports`
- `ReportBaselineMean`
- `ReportBaselineStdDev`
- `ReportBaselineWeeks`
- `RecentProbeTotal`
- `RecentProbeFailures`
- `ProbeBaselineFailureRate`
- `ProbeBaselineSamples`

## Time windows and constants

Current thresholds in `algorithm/status.go`:

- report window: 30 minutes
- recent probe window: latest 5 probe results
- minimum report baseline maturity: 4 weeks
- cold-start report outage threshold: 15 reports
- mature report outage floor: at least 3 reports
- mature report outage z-score threshold: 3.0
- report support z-score threshold for probe escalation: 1.0
- minimum recent probe samples before probe evaluation: 3
- minimum mature probe baseline samples: 20
- fallback probe failure threshold on immature baselines: 80%

These boundaries are exercised directly in `algorithm/status_test.go`.

## User-report outage path

### Cold-start behavior

If `ReportBaselineWeeks < 4`, the service does not yet have enough history for statistical comparison. In that case:

- `RecentReports >= 15` yields `Outage`
- otherwise the report path does nothing

### Mature baseline behavior

Once `ReportBaselineWeeks >= 4`, the current report count is compared to the current hour-of-week baseline:

```text
z = (RecentReports - ReportBaselineMean) / max(ReportBaselineStdDev, 1.0)
```

The strong report outage signal triggers only when both conditions hold:

- `z >= 3.0`
- `RecentReports >= 3`

## Probe degradation path

### Recent sample guard

If fewer than 3 recent probe samples exist, the probe path does nothing.

### Immature probe baseline

If the service has fewer than 20 probe baseline samples for the current hour-of-week bucket:

- recent failure rate `>= 0.8` yields `Degraded`

### Mature probe baseline

With a mature probe baseline, the probe failure threshold is:

```text
threshold = min(max(0.6, ProbeBaselineFailureRate + 0.4), 0.95)
```

If the recent probe failure rate meets or exceeds that threshold, the service becomes `Degraded`.

Probe failures alone do not produce `Outage`.

## Probe-plus-report escalation

When probes are already degraded, the algorithm can promote the service to `Outage` using a softer report anomaly signal.

This support signal is available only when `ReportBaselineWeeks >= 4` and uses the same report baseline inputs:

```text
z = (RecentReports - ReportBaselineMean) / max(ReportBaselineStdDev, 1.0)
```

The support anomaly is true when both conditions hold:

- `z >= 1.0`
- `RecentReports > ReportBaselineMean`

That means:

- strong report anomalies still create `Outage` directly
- degraded probes plus a real report anomaly also create `Outage`
- degraded probes without report support stay `Degraded`
- cold-start services do not get this assist path

## Final precedence

Status is resolved in this order:

1. `Outage` if the strong report outage rule fires
2. `Outage` if probes are degraded and the report support anomaly is true
3. `Degraded` if probes are degraded and neither outage rule fired
4. `Operational` otherwise

## Incident behavior

The incident worker recalculates status once per minute for active services.

- incidents open only on transitions into `Outage`
- incidents resolve as soon as status leaves `Outage`
- `Degraded` does not create incident records

## Where recent probe samples come from

Recent probe inputs are produced by the separate `probe` runtime mode.

- `workers/probe.go` claims due enabled probe configs for active services
- each run writes one `probe_results` row with status code, latency, and normalized failure type
- request-time probe summaries read the latest rows through `storage.GetRecentProbeStats` and `storage.GetProbeServiceDetail`
- raw probe history is retained for 30 days before worker-mode cleanup

Failure-type normalization is implemented in `workers/probe_failure.go` and `structs/probe_failure.go`.

## Where baselines come from

Baselines are refreshed in `workers/baseline.go` via `storage.RefreshAllBaselines`.

Report baseline generation in `storage/baselines.go`:

- includes active services only
- looks back from now up to 6 months, capped by the service `CreatedAt`
- snaps time to 30-minute windows
- generates zero-report windows as well as non-zero ones
- rolls those windows into `hour_of_week` buckets (`0..167`, UTC)
- stores mean reports, standard deviation, and weekly sample count per bucket

Probe baseline generation uses the same hour-of-week buckets over `probe_results` and stores:

- average failure rate
- probe sample count
- median success latency
- latency sample count

If probe tables are unavailable, report baselines still continue and probe data simply behaves like no probe signal.

## Test coverage

`algorithm/status_test.go` covers:

- cold-start vs mature report outage behavior
- softer report support anomaly behavior
- insufficient recent probe samples
- immature probe threshold behavior
- mature probe threshold floor, linear, and cap regimes
- probe-only degraded behavior
- degraded probe plus report escalation behavior

Relevant files:

- `algorithm/status.go`
- `algorithm/status_test.go`
- `utils/api-status.go`
- `utils/api-builders.go`
- `utils/probes.go`
- `workers/incidents.go`
- `workers/probe.go`
- `workers/probe_failure.go`
- `storage/baselines.go`
- `storage/probes.go`
