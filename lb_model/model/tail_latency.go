package model

type tailLatency struct {
	percentile percentile
	tlProb     string
}

func newTailLatency(p percentile, tlp string) *tailLatency {
	return &tailLatency{
		percentile: p,
		tlProb:     tlp,
	}
}

func (tl *tailLatency) getTailLatencyThreshold() float64 {
	switch tl.tlProb {
	case "p50":
		return tl.percentile.p50
	case "p95":
		return tl.percentile.p95
	case "p99":
		return tl.percentile.p99
	case "p999":
		return tl.percentile.p999
	case "p9999":
		return tl.percentile.p9999
	default:
		return 0
	}
}
