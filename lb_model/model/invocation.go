package model

import (
	"strconv"
)

type Invocation struct {
	te traceEntry
	im invocationMetadata
}

func newInvocation(id string, te traceEntry) *Invocation {
	return &Invocation{
		te: te,
		im: invocationMetadata{
			invocationId: id,
		},
	}
}

// CopyInvocation makes a hedge copy of i. The copy gets its own private
// parsedRow (a value copy, not a shared pointer) since SetDuration and
// SetReplicaID are only ever called on copies, to re-target them to the
// alternate replica the load balancer picked — mutating a row shared with
// the original (and with every other simulation run over the same trace)
// would corrupt it.
func CopyInvocation(i *Invocation) *Invocation {
	rowCopy := *i.te.row
	return &Invocation{
		te: traceEntry{
			row:         &rowCopy,
			tailLatency: i.te.tailLatency,
		},
		im: invocationMetadata{
			invocationId: i.im.invocationId,
			responseTime: i.im.responseTime,
			srcInvoc:     i,
		},
	}
}

func (i *Invocation) IsTailLatency() bool {
	return i.GetDuration() > i.GetTailLatencyThreshold()
}

func (i *Invocation) IsCopy() bool {
	return i.im.srcInvoc != nil
}

func (i *Invocation) UpdateResponse(hopResponse float64) {
	i.im.responseTime += hopResponse
}

func (i *Invocation) UpdateTechniqueResponseTime(iCopy *Invocation) {
	i.im.techniqueResponseTime = iCopy.im.processedTs - i.im.forwardedTs
}

func (i *Invocation) SetProcessedTs(pt float64) {
	i.im.processedTs = pt
}

func (i *Invocation) SetForwardedTs(ft float64) {
	i.im.forwardedTs = ft
}

// SetDuration is only ever called on a hedge copy (see CopyInvocation),
// whose row is private, never on an original invocation, which shares its
// row with every other simulation run over the same trace.
func (i *Invocation) SetDuration(nd float64) {
	i.te.row.duration = nd
}

// SetReplicaID re-targets a hedge copy to the replica that will actually
// process it — set once the load balancer has picked an alternate replica,
// so output/debugging reflects where the copy really ran. The latency it
// samples still comes from the original tenant+replica's own distribution
// (see technique.go), independent of this. Like SetDuration, only ever
// called on a copy's private row.
func (i *Invocation) SetReplicaID(id string) {
	i.te.row.replicaID = id
}

func (i *Invocation) GetTailLatencyThreshold() float64 {
	return i.te.tailLatency.getTailLatencyThreshold()
}

func (i *Invocation) GetTenantID() string {
	return i.te.row.tenantID
}

func (i *Invocation) GetReplicaID() string {
	return i.te.row.replicaID
}

func (i *Invocation) GetDuration() float64 {
	return i.te.row.duration
}

func (i *Invocation) GetStartTS() float64 {
	return i.te.row.startTS
}

func (i *Invocation) GetSrcInvoc() *Invocation {
	return i.im.srcInvoc
}

func (i *Invocation) getOutPut() []string {
	endTS := i.te.row.startTS + i.te.row.duration
	return []string{
		i.te.row.tenantID,
		i.te.row.replicaID,
		i.im.invocationId,

		strconv.FormatFloat(endTS, 'f', -1, 64),
		strconv.FormatFloat(i.te.row.startTS, 'f', -1, 64),
		strconv.FormatFloat(i.te.tailLatency.getTailLatencyThreshold(), 'f', -1, 64),

		strconv.FormatFloat(i.te.row.duration, 'f', -1, 64),
		strconv.FormatFloat(i.im.responseTime, 'f', -1, 64),
		strconv.FormatFloat(i.im.techniqueResponseTime, 'f', -1, 64),
	}
}
