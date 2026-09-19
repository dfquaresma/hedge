package common

import (
	"math/rand"

	"github.com/agoussia/godes"
	"github.com/dfquaresma/hedge/lb_model/model"
)

// technique implements the tail-latency mitigation applied by a replica.
// Supported values: "baseline" (no-op) and "hedged_request" (send a copy to
// a different replica, chosen by the router's load-balancer policy, once
// the original exceeds its tail-latency threshold; cancelling whichever
// finishes last). The copy targets another replica rather than another
// thread on the same one, since hedging within the replica that's already
// slow doesn't protect against replica-level causes (a noisy neighbor, a GC
// pause on that specific backend) and only adds load to it.
type technique struct {
	*godes.Runner
	replica *replica
	router  *router
	config  string
	rng     *rand.Rand
}

func newTechnique(rep *replica, t string, r *router) *technique {
	return &technique{
		Runner:  &godes.Runner{},
		replica: rep,
		router:  r,
		config:  t,
		// Deterministic per-replica seed keeps runs reproducible.
		rng: rand.New(rand.NewSource(int64(len(rep.replicaID)) + 42)),
	}
}

// newLatency resamples a service time from the empirical distribution of the
// invocation's tenant+replica group — the copy is a brand-new execution, so
// it gets its own draw rather than reusing the original's duration.
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

	altReplica := t.router.pickHedgeReplica(t.replica.replicaID)
	if altReplica == nil {
		// No alternate replica exists (e.g. the trace has only one) —
		// there's nowhere else to send a copy.
		return false, 0
	}

	iCopy := model.CopyInvocation(i)
	iCopy.SetForwardedTs(godes.GetSystemTime())
	// Sampled from the original tenant+replica's own empirical
	// distribution — a random historical latency of that tenant's, not the
	// alternate replica's profile, which may look nothing like it.
	iCopy.SetDuration(t.newLatency(i.GetTenantID() + i.GetReplicaID()))
	iCopy.SetReplicaID(altReplica.replicaID)

	th := altReplica.getAvailableThread()
	if th == nil {
		// Alternate replica is at its thread cap: queue the copy there like
		// any other invocation instead of dropping it (dispatched in
		// setAvailable once a thread frees up). Its eventual finish time
		// will include that wait, which we can't know yet, so skip the
		// early-cancel decision for this dispatch — the copy's response is
		// still recorded normally via processResponse once it completes.
		altReplica.forward(iCopy)
		return false, 0
	}
	copyThreadIsWarm := th.getRequestCount() != 0
	th.process(iCopy)

	// The original is cancelled if the copy is faster, but only when the
	// copy landed on a warm thread and thus pays no warm-up penalty.
	if i.GetDuration() > iCopy.GetDuration() && copyThreadIsWarm {
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
