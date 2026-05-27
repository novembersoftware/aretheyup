# Status Algorithm

## Purpose

The algorithm decides whether a service is `Operational` or `Issues Detected` using two signal families:

- recent user outage reports
- recent probe failures

The implementation lives in `algorithm/status.go`. The same logic is reused by API responses and the incident worker through `utils/api-status.go` and `workers/incidents.go`.

## Inputs

The algorithm accepts `algorithm.Signals`:

- `RecentReports`: user reports in the current rolling window
- `ReportBaselineMean`
- `ReportBaselineStdDev`
- `ReportBaselineWeeks`
- `RecentProbeTotal`
- `RecentProbeFailures`
- `ProbeBaselineFailureRate`
- `ProbeBaselineSamples`

The top-level rule is OR logic: one strong user-report signal or one strong probe signal is enough to produce `Issues Detected`.

## Time windows and constants

Current thresholds in `algorithm/status.go`:

- report window: 30 minutes
- recent probe window: latest 5 probe results
- minimum baseline maturity for report anomalies: 4 weeks
- cold-start report threshold: 15 reports
- minimum absolute reports on the mature path: 3
- report z-score threshold: 3.0
- minimum recent probe samples before probe evaluation: 3
- minimum mature probe baseline samples: 20
- fallback probe failure threshold on immature baselines: 80%

These boundaries are exercised directly in `algorithm/status_test.go`.

## User-report path

### Cold-start behavior

If `ReportBaselineWeeks < 4`, the service is treated as not having enough history for statistical comparison. In that case:

- `RecentReports >= 15` triggers `Issues Detected`
- otherwise the user-report path stays operational

This prevents brand-new services from flipping on weak or noisy data.

### Mature baseline behavior

Once `ReportBaselineWeeks >= 4`, the current report count is compared against the baseline for the current hour-of-week bucket:

```text
z = (RecentReports - ReportBaselineMean) / max(ReportBaselineStdDev, 1.0)
```

The `max(..., 1.0)` floor avoids overreacting to very small baseline variance.

The user-report signal triggers only when both conditions hold:

- `z >= 3.0`
- `RecentReports >= 3`

That second guard prevents tiny counts from tripping the service even when the z-score is numerically large.

## Probe path

### Recent sample guard

If fewer than 3 recent probe samples exist, the probe path does nothing.

### Immature probe baseline

If the service has fewer than 20 probe baseline samples for the current hour-of-week bucket, the algorithm uses a strict fallback rule:

- recent failure rate `>= 0.8` triggers `Issues Detected`

### Mature probe baseline

With a mature probe baseline, the failure threshold is:

```text
threshold = min(max(0.6, ProbeBaselineFailureRate + 0.4), 0.95)
```

That means:

- the threshold never drops below 60%
- it is normally the baseline failure rate plus 40 points
- it never rises above 95%

The probe path triggers when the recent failure rate meets or exceeds that threshold.

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

If probe tables are unavailable, report baselines still continue and probe data simply behaves like no probe signal.

## Operational use in the app

### API responses

List and detail routes gather recent counts and baseline rows, then call `utils.DetermineStatus`. See:

- `utils/api-builders.go`
- `utils/api-status.go`
- `api/routes/services.go`

### Incident tracking

The incident worker recalculates status once per minute for active services and opens or resolves incidents on status transitions. See `workers/incidents.go`.

## Test coverage

`algorithm/status_test.go` covers:

- cold-start vs mature user-report behavior
- z-score boundary cases
- standard-deviation floor behavior
- insufficient recent probe samples
- immature probe baseline threshold behavior
- mature probe baseline floor, linear, and cap regimes
- top-level OR behavior across user and probe signals

Relevant files:

- `algorithm/status.go`
- `algorithm/status_test.go`
- `utils/api-status.go`
- `utils/api-builders.go`
- `workers/baseline.go`
- `workers/incidents.go`
- `storage/baselines.go`
