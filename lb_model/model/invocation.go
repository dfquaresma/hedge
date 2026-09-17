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
			appID:       i.te.appID,
			funcID:      i.te.funcID,
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

func (i *Invocation) GetTailLatencyThreshold() float64 {
	return i.te.tailLatency.getTailLatencyThreshold()
}

func (i *Invocation) GetAppID() string {
	return i.te.appID
}

func (i *Invocation) GetFuncID() string {
	return i.te.funcID
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
		i.te.appID,
		i.te.funcID,
		i.im.invocationId,

		strconv.FormatFloat(i.te.endTS, 'f', -1, 64),
		strconv.FormatFloat(i.te.startTS, 'f', -1, 64),
		strconv.FormatFloat(i.te.tailLatency.getTailLatencyThreshold(), 'f', -1, 64),

		strconv.FormatFloat(i.te.duration, 'f', -1, 64),
		strconv.FormatFloat(i.im.responseTime, 'f', -1, 64),
		strconv.FormatFloat(i.im.techniqueResponseTime, 'f', -1, 64),
	}
}
