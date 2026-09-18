package common

// loadBalancer picks which replica a hedge copy is routed to, given the
// replica the original request is already running on. Hedging within that
// same replica doesn't protect against replica-level slowness (a noisy
// neighbor or a GC pause on that specific backend) and, now that a
// replica's thread pool is shared across tenants, it competes with other
// tenants for that same replica's capacity instead of spreading load — so
// the copy always targets a different replica.
type loadBalancer interface {
	// pick returns the replica to use for a hedge copy, skipping exclude
	// (the invocation's own replica). Returns nil if no alternate exists
	// (e.g. the trace has only one replica).
	pick(exclude string) *replica
}

// roundRobinBalancer cycles through every replica known in the trace, in a
// fixed order. All replicas are created upfront (see NewRouter) from the
// trace's full, pre-parsed set of replicaIDs, so the rotation is complete
// and stable from the very first invocation — nothing is discovered
// mid-replay.
type roundRobinBalancer struct {
	replicas []*replica
	next     int
}

func newRoundRobinBalancer(replicas []*replica) *roundRobinBalancer {
	return &roundRobinBalancer{replicas: replicas}
}

func (b *roundRobinBalancer) pick(exclude string) *replica {
	n := len(b.replicas)
	if n == 0 {
		return nil
	}
	for i := 0; i < n; i++ {
		r := b.replicas[b.next]
		b.next = (b.next + 1) % n
		if r.replicaID != exclude {
			return r
		}
	}
	// Every known replica is the excluded one — a single-replica trace.
	return nil
}
