# Database Optimization Plan

This plan breaks the database performance work into implementation tickets. The goal is to keep probes as a core product feature while making database writes, reads, baseline refreshes, and retention bounded and predictable.

## Ticket 1: Split API, Probe, and Background Worker Modes

**Goal**

Separate web serving from recurring database work so API containers do not also run baseline refreshes and incident reconciliation.

**Context**

API mode currently starts the baseline refresher and incident tracker. That makes every API process capable of running heavy background DB queries.

**Scope**

- Add a new runtime mode, for example `worker`.
- Keep `api` mode focused on HTTP serving.
- Keep `probe` mode focused on synthetic probe execution.
- Move baseline refresh, incident reconciliation, status snapshot refresh, and cleanup jobs into `worker` mode.
- Update Docker Compose profiles so production can run `api`, `probe`, and `worker` separately.

**Implementation Notes**

- Update `utils/parse-flags.go` to support the new mode.
- Update `main.go` so `apiMode` does not start background workers.
- Add `workerMode` for database background jobs.
- Keep one worker instance active, or add Postgres advisory locks around jobs that must not overlap.

**Acceptance Criteria**

- Starting `api` runs only the HTTP server.
- Starting `probe` runs only probe execution.
- Starting `worker` runs baseline, incident, status snapshot, and cleanup jobs.
- Running multiple API containers no longer multiplies background database load.

**Validation**

- Run `go test ./...`.
- Start each mode locally and confirm logs show only the expected work.
- In production, confirm only one worker process is active.

## Ticket 2: Add Production-Safe Indexes for Existing Hot Queries

**Goal**

Make the current schema safer before deeper application changes land.

**Context**

The `probe_results` table has millions of rows and the app frequently asks for latest rows, latest failures, and old rows for deletion.

**Scope**

Add indexes concurrently in a migration or one-off production SQL script:

```sql
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_probe_results_service_created_desc
ON probe_results (service_id, created_at DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_probe_results_service_failed_created_desc
ON probe_results (service_id, created_at DESC)
WHERE success = false;

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_probe_results_created_at
ON probe_results (created_at);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_user_reports_service_created_desc
ON user_reports (service_id, created_at DESC);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_incidents_active_by_service
ON incidents (service_id)
WHERE resolved_at IS NULL;
```

**Acceptance Criteria**

- Latest probe history queries use `idx_probe_results_service_created_desc`.
- Latest probe failure queries use `idx_probe_results_service_failed_created_desc`.
- Cleanup can find old rows by `created_at`.
- Recent report count queries use the service/time index.

**Validation**

- Run `EXPLAIN (ANALYZE, BUFFERS)` on the main probe and report queries before and after.
- Confirm index creation does not lock production writes.

## Ticket 3: Replace Per-Service Probe Intervals With a Global Cadence

**Goal**

Keep every service probed, but reduce write volume and scheduling complexity.

**Context**

Probe configs currently include `interval_seconds`. All services must have probes, but the interval can move to a global product setting.

**Scope**

- Set a global probe interval of 5 minutes.
- Stop using `probe_configs.interval_seconds` for scheduling.
- Optionally remove the column in a later cleanup migration after rollout.
- Add jitter so newly created or backfilled configs spread across the 5-minute window.
- Keep all active services with enabled probe configs.

**Implementation Notes**

- Add a constant or config value such as `PROBE_INTERVAL_SECONDS`, default `300`.
- Update `DefaultProbeConfig` to set `NextRunAt` with jitter.
- Update `nextProbeRunAt` to use the global interval.
- Preserve timeout, method, URL, and expected status in `probe_configs`.

**Acceptance Criteria**

- Every active service still has a probe config.
- Probes run about every 5 minutes per service.
- Restarting the probe worker does not make every service due at the same instant.
- Probe write volume drops by roughly 80% compared with 60-second probes.

**Validation**

- Unit test `nextProbeRunAt`.
- Confirm due probe distribution is spread over time.
- Monitor probe rows written per hour after deployment.

## Ticket 4: Add Redis Caching for Expensive API Reads

**Goal**

Use Redis to reduce repeated service-card and list-page database reads.

**Context**

Service detail currently performs multiple reads for recent reports, baseline, probe stats, probe detail, histograms, regional counts, incidents, and daily report counts.

**Scope**

- Cache service list responses for a short TTL.
- Cache service detail/card data by service slug for a short TTL.
- Cache derived probe presentation data.
- Invalidate or shorten cache after user report submission.

**Implementation Notes**

- Keep TTLs short, for example 10-30 seconds for list/status data and 30-60 seconds for historical detail blocks.
- Use versioned keys, for example:
  - `services:list:v3:page:{page}`
  - `service:card:v1:{slug}`
  - `service:detail:history:v1:{service_id}`
- Do not cache rate-limit-specific fields like `CanReport` unless keyed by requester.

**Acceptance Criteria**

- Repeated list/detail requests hit Redis instead of re-running all DB queries.
- Report submission still updates the visible card quickly.
- Cache failures fall back to DB without breaking responses.

**Validation**

- Add tests for cache key behavior where practical.
- Add debug metrics/logs for cache hit and miss rates.
- Load test repeated service detail requests before and after.

## Ticket 5: Add Derived Probe State and Recent History Tables

**Goal**

Stop using raw `probe_results` as the live read model.

**Context**

The app only needs current probe state, recent history, recent failure counts, and compact aggregates for most user-facing flows.

**Scope**

Create new tables:

```sql
service_probe_states
- service_id primary key
- last_checked_at
- last_success_at
- last_failure_at
- last_success_status_code
- last_failure_status_code
- last_response_time_ms
- recent_probe_total
- recent_probe_failures
- recent_window_updated_at
- created_at
- updated_at

probe_recent_results
- id primary key
- service_id
- checked_at
- success
- status_code
- response_time_ms
- failure_type
- error_message
- created_at
```

**Implementation Notes**

- On probe completion, update `service_probe_states`.
- Insert into `probe_recent_results`.
- Keep only the latest 50-100 recent rows per service in `probe_recent_results`.
- Update API probe detail reads to use these derived tables.
- Continue writing raw `probe_results` during rollout until rollups and retention are proven.

**Acceptance Criteria**

- Service cards and service detail no longer query `probe_results` for current probe state.
- Recent probe history reads are bounded by the capped table.
- The latest failure timestamp comes from `service_probe_states`, not a raw-table sort.

**Validation**

- Add storage tests for probe completion updating state and recent history.
- Compare old and new API responses in development.
- Confirm query plans stay bounded regardless of raw table size.

## Ticket 6: Add Probe Hourly Rollups for Baselines

**Goal**

Make probe baselines read compact hourly aggregate data instead of scanning raw probe history.

**Context**

Baseline refresh currently groups raw `probe_results` by hour-of-week and calculates failure rates across historical rows.

**Scope**

Create a `probe_hourly_rollups` table:

```sql
probe_hourly_rollups
- service_id
- bucket_start
- hour_of_week
- total_count
- failure_count
- success_latency_sum_ms
- success_latency_count
- min_latency_ms
- max_latency_ms
- created_at
- updated_at
- unique(service_id, bucket_start)
```

**Implementation Notes**

- On probe completion, upsert the current hour bucket.
- Baseline refresh should aggregate from `probe_hourly_rollups`.
- Remove the expensive median latency calculation unless it becomes product-critical.
- Keep failure rate and sample count, since the algorithm uses those.

**Acceptance Criteria**

- Baseline refresh no longer scans `probe_results`.
- Probe baseline sample counts and failure rates match the old logic within acceptable tolerance.
- Baseline refresh runtime is stable as raw probe history grows.

**Validation**

- Backfill rollups from existing raw rows once.
- Compare generated `service_baselines` before and after switching.
- Run `EXPLAIN` against the new baseline query.

## Ticket 7: Add Service Status Snapshots

**Goal**

Make API list and incident reconciliation read precomputed current status instead of recomputing from raw inputs on every path.

**Context**

The app repeatedly calculates current service status from recent reports, baseline rows, and recent probe stats.

**Scope**

Create a `service_statuses` table:

```sql
service_statuses
- service_id primary key
- status
- recent_reports
- recent_probe_total
- recent_probe_failures
- baseline_mean_reports
- computed_at
- created_at
- updated_at
```

**Implementation Notes**

- Add a worker job that refreshes status snapshots every 30-60 seconds.
- Refresh the affected service after a user report submission.
- API list pages should join or read `service_statuses`.
- Incident reconciliation should use `service_statuses` instead of recalculating every service.

**Acceptance Criteria**

- List pages do not query `probe_results`.
- Incident reconciliation does not query `probe_results`.
- Service detail can still show rich history, but current status comes from a status snapshot.
- Status freshness is visible through `computed_at`.

**Validation**

- Add tests comparing snapshot status to existing `DetermineStatus` behavior.
- Confirm list endpoint query count and latency drop.
- Confirm incident open/resolve behavior remains correct.

## Ticket 8: Implement Raw Probe Retention and Purge Strategy

**Goal**

Keep enough probe evidence for debugging without allowing raw history to dominate the database.

**Context**

`probe_results` has grown past 4 million rows. After derived tables exist, raw rows should not be long-lived source data.

**Scope**

- Keep recent success history in `probe_recent_results`.
- Keep recent failures longer than successes.
- Reduce raw `probe_results` retention to 24-72 hours, or remove raw success rows once derived writes are proven.
- Delete old raw data in batches.
- Run `VACUUM ANALYZE` after large purges.

**Implementation Notes**

Suggested policy:

- `probe_recent_results`: latest 50-100 rows per service.
- Raw `probe_results` successes: 24 hours.
- Raw `probe_results` failures: 7-14 days.
- Rollups: 6-12 months, matching baseline needs.

**Acceptance Criteria**

- Raw `probe_results` row count stays bounded.
- Recent failures are still available for debugging.
- Recent success history remains available for UI display.
- Cleanup jobs do not lock the table for long periods.

**Validation**

- Test cleanup on a database copy.
- Monitor row counts and table size after deployment.
- Confirm `VACUUM ANALYZE` updates planner stats.

## Ticket 9: Backfill and Migration Rollout

**Goal**

Move to the new model without breaking live status pages.

**Scope**

- Create new tables and indexes.
- Backfill `probe_hourly_rollups` from existing `probe_results`.
- Backfill `service_probe_states` from latest probe rows.
- Backfill `probe_recent_results` from latest rows per service.
- Deploy dual-write probe completion.
- Switch API reads to derived tables.
- Switch baseline refresh to rollups.
- Switch incident reconciliation to status snapshots.
- Enable retention cleanup.

**Acceptance Criteria**

- New read paths match old read paths before cutover.
- Cutover can be rolled back by reading old tables while dual-write remains active.
- Production row counts and memory stabilize after retention cleanup.

**Validation**

- Run backfill on staging or a production snapshot first.
- Compare old/new responses for a sample of healthy, degraded, and outage services.
- Track DB memory, slow queries, rows written per minute, and table sizes before and after.

## Ticket 10: Add Observability for Database Load

**Goal**

Make regressions visible before the database becomes slow again.

**Scope**

- Log duration and row counts for background jobs.
- Track probe write rate.
- Track baseline refresh duration.
- Track status snapshot refresh duration.
- Track Redis cache hit/miss rates.
- Track raw and derived table row counts.

**Acceptance Criteria**

- Slow background jobs are visible in logs.
- Cache effectiveness is measurable.
- Probe table growth is visible.
- Operators can tell whether API, probe, or worker mode is causing load.

**Validation**

- Confirm logs include job duration and affected rows.
- Add simple SQL checks or dashboard panels for table sizes and row counts.

