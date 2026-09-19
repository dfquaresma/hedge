package model

// Config holds the parameters of a single simulation run.
type Config struct {
	ForwardLatency    float64
	ColdStartDuration float64
	MaxThreads        int // 0 = unlimited concurrent threads per replica
	TailLatencyProb   string
	Technique         string
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

type traceEntry struct {
	tenantID  string
	replicaID string
	groupSize int64
	startTS   float64
	duration  float64
	endTS     float64

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
