# lb_model — load-balancer trace replayer

A discrete-event simulator (built on [godes](https://github.com/agoussia/godes))
that replays AWS ALB/ELB access-log traces to evaluate tail-latency mitigation
techniques against real production workloads. It follows the
replayer → router → provisioner → replica architecture of
[faas-simulator](https://github.com/dfquaresma/faas-simulator)'s
`replica_model`, adapted for load-balancer traces:

- **Generic CSV input.** Columns are resolved by name via a config-defined
  mapping, so any CSV derived from ALB access logs works without
  preprocessing — extra columns are ignored.
- **Percentiles computed at load time.** Tail-latency thresholds (P50–P99.99)
  are derived per `app+func` group from the trace itself; no external
  percentile preprocessing step is required.
- **Additive warm-up instead of FaaS cold start.** LB traces carry no
  cold-start information, so a fresh replica optionally pays a configurable
  `coldStartDuration` penalty on its first request (0 disables it).
- **Timestamps** may be epoch seconds (float) or RFC3339/`YYYY-MM-DD HH:MM:SS`
  strings; they are normalized to start at zero.

## Architecture

```
replayer   reads the chronological trace, advances the simulation clock
   └─> router          one provisioner per app+func group
          └─> provisioner   LIFO pool of warm replicas, idletime-based scale-down
                 └─> replica    one concurrency slot; serves one request at a time
```

A *replica* models a backend **concurrency slot**, not a machine: a node
serving N concurrent requests is represented by N replicas. Replica counts in
the outputs therefore read as "busy slots over time".

Techniques:

- `baseline` — replay as-is.
- `hedged_request` — when a request runs past its tail-latency threshold
  (the `tailLatencyProb` percentile), dispatch a copy to another replica with
  a service time resampled from the group's empirical distribution; whichever
  finishes last is cancelled.

## Threshold scope: heterogeneity-aware vs blind hedging

Real multi-tenant traces are heterogeneous — latency distributions differ per
tenant. `thresholdScope` controls which distribution defines the hedging
threshold, enabling a three-way comparison on the same trace:

| Scenario | Config | Threshold |
|---|---|---|
| no hedge | `technique: baseline` | — |
| blind hedge | `hedged_request` + scope `global` | percentile of the **whole trace** |
| heterogeneity-aware hedge | `hedged_request` + scope `per_group` | percentile of the request's **own app+func group** |

With a heterogeneous trace, a global P95 sits between the tenants' individual
P95s: fast tenants practically never reach it (their tail is never hedged),
while for slow tenants it may fall below their median (over-hedging, inflating
system load). The `per_group` scope calibrates the trigger per tenant.

Only the threshold changes with scope — hedged copies always resample from
the request's own group distribution, since a tenant's latency profile is a
property of the workload, not of the policy. `baseline` runs once regardless
of the configured scopes (its results are scope-independent), and omitting
`thresholdScope` defaults to `["per_group"]`.

To compare strictly per tenant (ignoring request classes), map `func` to the
same column as `app` in the column mapping.

## Input format

Any CSV with a header row containing at least four columns, mapped in
`config.json`:

| Config key        | Meaning                            | Example ALB-derived column |
|-------------------|------------------------------------|----------------------------|
| `app`             | tenant / workload identifier       | `instance_id`              |
| `func`            | request class within the tenant    | `request_type`             |
| `startTimestamp`  | request start time                 | `request_creation_time`    |
| `duration`        | backend latency in seconds         | `target_processing_time`   |

Rows with non-positive/unparsable duration are dropped, as are `app+func`
groups with fewer than `minGroupSize` samples (percentile thresholds from tiny
groups are meaningless — for real traces use at least a few thousand).

`traces/alb/sample-trace.csv` is a small **fully synthetic** example of the
expected shape (fictitious tenants, RFC 5737 documentation IPs, generated
latencies). Its three tenants have deliberately different latency profiles —
fast/tight, slow/heavy-tailed and bimodal — so the threshold-scope comparison
is visible even on the sample. No production data is committed to this
repository — point `tracePath` at your local trace instead.

## Running

```bash
cd lb_model/
go run .
```

Each section of `config.json` is one trace; the parameter grid
(`tailLatencyProb` × `idletime` × `technique`) is swept per section. Per run,
three CSVs are written to `outputPath`:

- `*-invocations.csv` — per-request `duration`, `responseTime` and
  `techniqueResponseTime`
- `*-replicas.csv` — per-replica `busyTime`, `upTime`, requests processed
- `*-provisioners.csv` — replica-count scaling timeline

plus a `replayer-stats.csv` with wall-clock time per run.
