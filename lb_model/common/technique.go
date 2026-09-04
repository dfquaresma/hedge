package common

import (
	"math/rand"

	"github.com/agoussia/godes"
	"github.com/dfquaresma/hedge/lb_model/model"
)

// technique implements the tail-latency mitigation applied by a provisioner.
// Supported values: "baseline" (no-op) and "hedged_request" (send a copy to
// another replica once the original exceeds its tail-latency threshold,
// cancelling whichever finishes last).
type technique struct {
	*godes.Runner
	provisioner *provisioner
	router      *router
	config      string
	rng         *rand.Rand
}

func newTechnique(p *provisioner, t string, r *router) *technique {
	return &technique{
		Runner:      &godes.Runner{},
		provisioner: p,
		router:      r,
		config:      t,
		// Deterministic per-provisioner seed keeps runs reproducible.
		rng: rand.New(rand.NewSource(int64(len(p.rpID)) + 42)),
	}
}

// newLatency resamples a service time from the empirical distribution of the
// invocation's app+func group — the copy is a brand-new execution, so it gets
// its own draw rather than reusing the original's duration.
func (t *technique) newLatency(id string) float64 {
	latencies := t.router.getDataSet().GetLatenciesOf(id)
	return latencies[t.rng.Intn(len(latencies))]
}

// trigger runs when an invocation has been executing for its technique delay
// and is still unfinished. It returns whether the current invocation should
// be cancelled and after how much additional time.
func (t *technique) trigger(i *model.Invocation) (bool, float64) {
	if t.config != "hedged_request" {
		return false, 0
	}

	if i.IsCopy() {
		// The copy is cancelled if it outlives the original.
		if i.GetDuration() > i.GetSrcInvoc().GetDuration() {
			return true, i.GetSrcInvoc().GetDuration()
		}
		return false, 0
	}

	iCopy := model.CopyInvocation(i)
	iCopy.SetForwardedTs(godes.GetSystemTime())
	iCopy.SetDuration(t.newLatency(i.GetAppID() + i.GetFuncID()))
	replica := t.provisioner.getAvailableReplica()
	copyReplicaIsWarm := replica.getRequestCount() != 0
	replica.process(iCopy)

	// The original is cancelled if the copy is faster, but only when the
	// copy landed on a warm replica and thus pays no warm-up penalty.
	if i.GetDuration() > iCopy.GetDuration() && copyReplicaIsWarm {
		return true, iCopy.GetDuration()
	}
	return false, 0
}

func (t *technique) getTechniqueDelay(i *model.Invocation) float64 {
	if t.config == "hedged_request" && !i.IsCopy() {
		return i.GetTailLatencyThreshold()
	}
	return 0
}

func (t *technique) processResponse(i *model.Invocation) {
	if i.IsCopy() {
		srcInvoc := i.GetSrcInvoc()
		srcInvoc.UpdateTechniqueResponseTime(i)
	}
}
