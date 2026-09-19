package common

import (
	"strconv"

	"github.com/agoussia/godes"
	"github.com/dfquaresma/hedge/lb_model/model"
)

// thread models one concurrency slot within a replica's thread pool: it
// serves a single request at a time. A replica handling N concurrent
// requests is represented by N threads, so thread counts read as "busy
// slots", not machines. A thread is not tied to a tenant — the same replica
// (and therefore the same pool of threads) is shared by every tenant routed
// to it, so a thread may serve different tenants' requests over its
// lifetime.
type thread struct {
	*godes.Runner
	arrivalCond    *godes.BooleanControl
	terminatedCond *godes.BooleanControl
	isBusy         *godes.BooleanControl
	arrivalQueue   *godes.FIFOQueue
	parent         *replica
	threadID       string
	cfg            model.Config
	startTS        float64
	shutdownTS     float64
	lastWorkTS     float64
	busyTime       float64
	upTime         float64
	reqsCount      int
}

func newThread(parent *replica, threadID string, cfg model.Config) *thread {
	return &thread{
		Runner:         &godes.Runner{},
		arrivalCond:    godes.NewBooleanControl(),
		terminatedCond: godes.NewBooleanControl(),
		isBusy:         godes.NewBooleanControl(),
		arrivalQueue:   godes.NewFIFOQueue(threadID),
		parent:         parent,
		threadID:       threadID,
		cfg:            cfg,
	}
}

func (t *thread) process(i *model.Invocation) {
	t.arrivalQueue.Place(i)
	t.isBusy.Set(true)
	t.arrivalCond.Set(true)
}

func (t *thread) Run() {
	t.startTS = godes.GetSystemTime()
	t.parent.notifyReadyness(godes.GetSystemTime())
	for {
		t.arrivalCond.Wait(true)
		if t.arrivalQueue.Len() > 0 {
			i := t.arrivalQueue.Get().(*model.Invocation)
			// A fresh slot pays a warm-up penalty on its first request
			// (cache warm-up, connection setup, JIT, ...). Unlike the FaaS
			// cold-start model, the penalty is additive and configured
			// globally, since LB traces carry no per-request cold info.
			if t.reqsCount == 0 && t.cfg.ColdStartDuration > 0 {
				i.SetDuration(i.GetDuration() + t.cfg.ColdStartDuration)
			}

			forwardLatency := t.cfg.ForwardLatency
			if forwardLatency != 0 {
				godes.Advance(forwardLatency)
				t.busyTime += forwardLatency
				i.UpdateResponse(forwardLatency)
			}

			delay := t.parent.getTechniqueDelay(i)
			dur := i.GetDuration()
			if dur-delay >= 0 {
				if delay != 0 {
					godes.Advance(delay)
					t.busyTime += delay
					i.UpdateResponse(delay)
				}
				dur = i.GetDuration() - delay // dur is now the surplus latency after the technique delay

				shouldCancel, timeToCancel := t.parent.triggerTechnique(i)
				if shouldCancel {
					godes.Advance(timeToCancel)
					t.busyTime += timeToCancel
					t.lastWorkTS = godes.GetSystemTime()

					if t.terminatedCond.GetState() {
						t.setUptimeStats()
						break
					}

					t.isBusy.Set(false)
					t.parent.setAvailable(t)
					continue
				}
			}

			godes.Advance(dur)
			t.busyTime += dur

			i.UpdateResponse(dur)

			t.lastWorkTS = godes.GetSystemTime()
			i.SetProcessedTs(t.lastWorkTS)

			t.parent.response(i)
			t.reqsCount += 1
		}

		if t.arrivalQueue.Len() == 0 {
			if t.terminatedCond.GetState() {
				t.setUptimeStats()
				break
			}
			t.arrivalCond.Set(false)
			t.isBusy.Set(false)
			t.parent.setAvailable(t)
		}
	}
}

func (t *thread) setUptimeStats() {
	t.shutdownTS = godes.GetSystemTime()
	t.parent.notifyTermination(t.shutdownTS)
	t.upTime = t.shutdownTS - t.startTS
}

func (t *thread) terminate() {
	t.terminatedCond.Set(true)
	t.arrivalCond.Set(true)
}

func (t *thread) getRequestCount() int {
	return t.reqsCount
}

func (t *thread) getOutPut() []string {
	return []string{
		t.threadID,
		t.parent.replicaID,
		strconv.FormatFloat(t.busyTime, 'f', -1, 64),
		strconv.FormatFloat(t.upTime, 'f', -1, 64),
		strconv.Itoa(t.reqsCount),
		strconv.FormatFloat(t.lastWorkTS, 'f', -1, 64),
		strconv.FormatFloat(t.startTS, 'f', -1, 64),
		strconv.FormatFloat(t.shutdownTS, 'f', -1, 64),
	}
}
