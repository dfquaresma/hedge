# lb_model — load-balancer trace replayer

A discrete-event simulator (built on [godes](https://github.com/agoussia/godes))
that replays AWS ALB/ELB access-log traces to evaluate tail-latency mitigation
techniques against real production workloads. It follows the
replayer → router → replica → thread architecture, adapted from
[faas-simulator](https://github.com/dfquaresma/faas-simulator)'s
`replica_model` for load-balancer traces:

- **Generic CSV input.** Columns are resolved by name via a config-defined
  mapping, so any CSV derived from ALB access logs works without
  preprocessing — extra columns are ignored.
- **Percentiles computed at load time.** Tail-latency thresholds (P50–P99.99)
  are derived per `tenant+replica` group from the trace itself; no external
  percentile preprocessing step is required.
- **Additive warm-up instead of FaaS cold start.** LB traces carry no
  cold-start information, so a fresh thread optionally pays a configurable
  `coldStartDuration` penalty on its first request (0 disables it).
- **Timestamps** may be epoch seconds (float) or RFC3339/`YYYY-MM-DD HH:MM:SS`
  strings; they are normalized to start at zero.

## Architecture

```
replayer   reads the chronological trace, advances the simulation clock
   └─> router          one replica per replicaID (shared by every tenant routed to it)
          └─> replica         bounded pool of threads, idletime-based scale-down
                 └─> thread       one concurrency slot; serves one request at a time
```

A *replica* is a physical backend instance identified by the trace's replica
column (e.g. `target_ip`) — looked up by `replicaID` alone, **not** scoped by
tenant, since one real backend serves whichever tenants get routed to it
(this is what lets the model capture multi-tenant contention on shared
infrastructure — a "noisy neighbor" saturating a replica's thread pool slows
down every tenant sharing it).

A *thread* models one **concurrency slot** within a replica, not a machine: a
replica serving N concurrent requests is represented by N threads. Thread
counts in the outputs read as "busy slots over time". `resourceProvisioner.maxThreads`
caps how many threads a replica may run at once (0 or omitted = unlimited,
the original unbounded-pool behaviour); once at the cap, incoming requests
queue for the next thread that frees up rather than spinning up a new one.

Techniques:

- `baseline` — replay as-is.
- `hedged_request` — when a request runs past its tail-latency threshold
  (the `tailLatencyProb` percentile), dispatch a copy to another thread in
  the same replica's pool, with a service time resampled from the group's
  empirical distribution; whichever finishes last is cancelled. If the
  replica is at its thread cap with none free, the copy is skipped rather
  than adding load to an already-saturated backend.

## Threshold scope: heterogeneity-aware vs blind hedging

Real multi-tenant traces are heterogeneous — latency distributions differ per
tenant. `thresholdScope` controls which distribution defines the hedging
threshold, enabling a three-way comparison on the same trace:

| Scenario | Config | Threshold |
|---|---|---|
| no hedge | `technique: baseline` | — |
| blind hedge | `hedged_request` + scope `global` | percentile of the **whole trace** |
| heterogeneity-aware hedge | `hedged_request` + scope `per_group` | percentile of the request's **own tenant+replica group** |

With a heterogeneous trace, a global P95 sits between the tenants' individual
P95s: fast tenants practically never reach it (their tail is never hedged),
while for slow tenants it may fall below their median (over-hedging, inflating
system load). The `per_group` scope calibrates the trigger per tenant. This
percentile grouping is a statistical calibration only — it is independent of
how replicas are addressed at runtime (always by `replicaID` alone, see
above).

Only the threshold changes with scope — hedged copies always resample from
the request's own group distribution, since a tenant's latency profile is a
property of the workload, not of the policy. `baseline` runs once regardless
of the configured scopes (its results are scope-independent), and omitting
`thresholdScope` defaults to `["per_group"]`.

To compare strictly per tenant (ignoring the grouping dimension), map
`replica` to the same column as `tenant` in the column mapping.

## Input format

Any CSV with a header row containing at least four columns, mapped in
`config.json`:

| Config key        | Meaning                                                                                          | Example column               |
|--------------------|--------------------------------------------------------------------------------------------------|-------------------------------|
| `tenant`           | tenant / workload identifier                                                                     | `instance_id`                 |
| `replica`          | the backend instance a request was routed to — also the grouping key for tail-latency thresholds and hedge-copy resampling | `target_ip` / `request_type`  |
| `startTimestamp`   | request start time                                                                                | `request_creation_time`       |
| `duration`         | backend latency in seconds                                                                         | `target_processing_time`      |

Rows with non-positive/unparsable duration are dropped, as are `tenant+replica`
groups with fewer than `minGroupSize` samples (percentile thresholds from tiny
groups are meaningless — for real traces use at least a few thousand).

`traces/alb/sample-trace.csv` is a small **fully synthetic** example of the
expected shape (fictitious tenants, RFC 5737 documentation IPs, generated
latencies). Its three tenants have deliberately different latency profiles —
fast/tight, slow/heavy-tailed and bimodal — so the threshold-scope comparison
is visible even on the sample. Its `replica` column maps to `request_type`
(no real backend-routing field in that synthetic trace). No production data
is committed to this repository — point `tracePath` at your local trace
instead.

The `alb_artorias` section demonstrates a trace where `replica` maps to
`target_ip` — the actual destination backend instance. See
`traces/processed/README.md` for the expected column shape (produced by the
`artorias-logs` prep pipeline).

## Running

```bash
cd lb_model/
go run .
```

Each section of `config.json` is one trace; the parameter grid
(`tailLatencyProb` × `idletime` × `maxThreads` × `technique`) is swept per
section. Per run, three CSVs are written to `outputPath`:

- `*-invocations.csv` — per-request `duration`, `responseTime` and
  `techniqueResponseTime`
- `*-threads.csv` — per-thread `busyTime`, `upTime`, requests processed
- `*-replicas.csv` — thread-count scaling timeline

plus a `replayer-stats.csv` with wall-clock time per run.
