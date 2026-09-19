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

func CopyInvocation(i *Invocation) *Invocation {
	return &Invocation{
		te: traceEntry{
			tenantID:    i.te.tenantID,
			replicaID:   i.te.replicaID,
			groupSize:   i.te.groupSize,
			startTS:     i.te.startTS,
			duration:    i.te.duration,
			endTS:       i.te.endTS,
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

func (i *Invocation) SetDuration(nd float64) {
	i.te.duration = nd
}

// SetReplicaID re-targets a hedge copy to the replica that will actually
// process it — set once the load balancer has picked an alternate replica,
// so output/debugging reflects where the copy really ran. The latency it
// samples still comes from the original tenant+replica's own distribution
// (see technique.go), independent of this.
func (i *Invocation) SetReplicaID(id string) {
	i.te.replicaID = id
}

func (i *Invocation) GetTailLatencyThreshold() float64 {
	return i.te.tailLatency.getTailLatencyThreshold()
}

func (i *Invocation) GetTenantID() string {
	return i.te.tenantID
}

func (i *Invocation) GetReplicaID() string {
	return i.te.replicaID
}

func (i *Invocation) GetDuration() float64 {
	return i.te.duration
}

func (i *Invocation) GetStartTS() float64 {
	return i.te.startTS
}

func (i *Invocation) GetSrcInvoc() *Invocation {
	return i.im.srcInvoc
}

func (i *Invocation) getOutPut() []string {
	return []string{
		i.te.tenantID,
		i.te.replicaID,
		i.im.invocationId,

		strconv.FormatFloat(i.te.endTS, 'f', -1, 64),
		strconv.FormatFloat(i.te.startTS, 'f', -1, 64),
		strconv.FormatFloat(i.te.tailLatency.getTailLatencyThreshold(), 'f', -1, 64),

		strconv.FormatFloat(i.te.duration, 'f', -1, 64),
		strconv.FormatFloat(i.im.responseTime, 'f', -1, 64),
		strconv.FormatFloat(i.im.techniqueResponseTime, 'f', -1, 64),
	}
}
