# traces/processed/

Place reduced/processed traces here (e.g. `processed_data.csv` produced by the
`artorias-logs` prep pipeline). CSV files in this directory are gitignored —
no production data is committed to this repository; point `tracePath` in
`lb_model/config.json` at your local file.

Expected shape for the `alb_artorias` config section:

| column | maps to | meaning |
|---|---|---|
| `instance_id` | `tenant` | tenant identifier |
| `target_ip` | `replica` | destination backend instance — the replica a request was routed to, also used as the grouping key for tail-latency thresholds and hedge-copy resampling |
| `request_creation_time` | `startTimestamp` | request arrival time (RFC3339, e.g. `2026-06-15T14:09:49.673Z`) |
| `target_processing_time` | `duration` | latency to replay/simulate, in seconds |
