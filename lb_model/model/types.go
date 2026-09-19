package model

// Config holds the parameters of a single simulation run.
//
// There is no cold-start/warm-up or forwarding-latency knob: a replica in
// this model represents an idealized downstream service — it scales its
// thread pool on demand (unbounded unless MaxThreads caps it) and a fresh
// thread costs nothing extra to spin up.
type Config struct {
	MaxThreads      int // 0 = unlimited concurrent threads per replica
	TailLatencyProb string
	Technique       string
}

// ColumnMapping names the trace columns holding each field the simulator
// needs. Any other column in the CSV is ignored, so traces with extra
// fields (status codes, byte counts, raw request lines, ...) can be
// replayed without preprocessing.
type ColumnMapping struct {
	Tenant         string
	Replica        string
	StartTimestamp string
	Duration       string
}

// traceEntry is an invocation's link back to trace data. row points into
// Trace.rows — the same backing array shared read-only by every simulation
// run of this trace, never reallocated per run — so only tailLatency
// (which depends on the run's tlProb/scope) needs to change between runs.
//
// A hedge copy gets its own private *parsedRow (see CopyInvocation) rather
// than sharing the original's, since its replicaID and duration are set
// after creation (SetReplicaID, SetDuration) and must not corrupt the
// shared row other runs still read.
type traceEntry struct {
	row         *parsedRow
	tailLatency *tailLatency
}

type invocationMetadata struct {
	invocationId string

	responseTime          float64
	techniqueResponseTime float64

	forwardedTs float64
	processedTs float64

	srcInvoc *Invocation
}

type percentile struct {
	p50   float64
	p95   float64
	p99   float64
	p999  float64
	p9999 float64
	p100  float64
}
