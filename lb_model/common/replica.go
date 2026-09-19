package common

import (
	"strconv"

	"github.com/agoussia/godes"
	"github.com/dfquaresma/hedge/lb_model/model"
)

// replica represents one backend instance identified by the trace's replica
// column (e.g. target_ip) — one object per replicaID, shared by every tenant
// routed to it (see router.go, which looks it up by replicaID alone). It
// owns a pool of threads (concurrency slots) bounded by cfg.MaxThreads and
// the hedging technique configured for the run.
//
// forward is fire-and-forget: it never blocks the caller (the replayer's
// single dispatch loop), even when every thread is busy and the pool is at
// its cap — the invocation is queued in pending and handed to a thread as
// soon as one frees up, in setAvailable.
type replica struct {
	*godes.Runner
	availableThreads *godes.LIFOQueue
	pending          []*model.Invocation
	replicaID        string
	cfg              model.Config
	threads          []*thread
	technique        *technique
	router           *router
	threadSeq        int
	activeThreads    int
}

func newReplica(replicaID string, cfg model.Config, r *router) *replica {
	rep := &replica{
		Runner:    &godes.Runner{},
		replicaID: replicaID,
		cfg:       cfg,
		router:    r,
	}
	rep.technique = newTechnique(rep, cfg.Technique, r)
	rep.availableThreads = godes.NewLIFOQueue(replicaID)
	return rep
}

func (r *replica) forward(i *model.Invocation) {
	if t := r.getAvailableThread(); t != nil {
		t.process(i)
		return
	}
	r.pending = append(r.pending, i)
}

func (r *replica) response(i *model.Invocation) {
	r.technique.processResponse(i)
}

// setAvailable is called by a thread that just finished its current request.
// If requests are waiting because the pool was at its cap, the thread picks
// up the oldest one immediately instead of going idle.
func (r *replica) setAvailable(t *thread) {
	if len(r.pending) > 0 {
		next := r.pending[0]
		r.pending = r.pending[1:]
		t.process(next)
		return
	}
	r.availableThreads.Place(t)
}

// getAvailableThread returns an idle thread, spins up a new one if the pool
// hasn't reached cfg.MaxThreads, or returns nil if it's at capacity — the
// caller (forward) then queues the invocation in pending. MaxThreads <= 0
// means unlimited, preserving the original unbounded-pool behaviour.
func (r *replica) getAvailableThread() *thread {
	for r.availableThreads.Len() > 0 {
		t := r.availableThreads.Get().(*thread)
		if t.terminatedCond.GetState() {
			continue
		}
		if r.cfg.Idletime < 0 || r.cfg.Idletime > godes.GetSystemTime()-t.lastWorkTS {
			return t
		}
		t.terminate()
	}
	if r.cfg.MaxThreads > 0 && r.activeThreads >= r.cfg.MaxThreads {
		return nil
	}
	r.activeThreads++
	r.threadSeq++
	t := newThread(r, r.replicaID+"-"+strconv.Itoa(r.threadSeq), r.cfg)
	godes.AddRunner(t)
	r.threads = append(r.threads, t)
	return t
}

func (r *replica) notifyReadyness(timestamp float64) {
	r.router.registerReplicaScaling(r.replicaID, 1, timestamp)
}

func (r *replica) notifyTermination(timestamp float64) {
	r.activeThreads--
	r.router.registerReplicaScaling(r.replicaID, -1, timestamp)
}

func (r *replica) triggerTechnique(i *model.Invocation) (bool, float64) {
	return r.technique.trigger(i)
}

func (r *replica) getTechniqueDelay(i *model.Invocation) float64 {
	return r.technique.getTechniqueDelay(i)
}

func (r *replica) terminate() {
	for _, t := range r.threads {
		t.terminate()
	}
}

func (r *replica) getOutPut() [][]string {
	res := [][]string{}
	for _, t := range r.threads {
		res = append(res, t.getOutPut())
	}
	return res
}
