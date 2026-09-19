package common

import (
	"strconv"

	"github.com/dfquaresma/hedge/lb_model/model"
)

// router dispatches each invocation to the replica identified by its
// replicaID alone — tenant is not part of the lookup key, since a physical
// replica is shared by every tenant routed to it (multi-tenant contention on
// shared infrastructure is exactly what this model is meant to capture).
//
// Every replica named in replicaIDs is created upfront, rather than lazily
// on first use, so the load balancer (used to pick a hedge copy's
// destination) has the complete, stable set of replicas from the very first
// invocation.
type router struct {
	replicas      map[string]*replica
	lb            loadBalancer
	dataset       *model.Dataset
	cfg           model.Config
	register      [][]string
	replicasCount int64
}

func NewRouter(dataset *model.Dataset, cfg model.Config, replicaIDs []string) *router {
	r := &router{
		replicas: make(map[string]*replica, len(replicaIDs)),
		dataset:  dataset,
		cfg:      cfg,
		register: [][]string{},
	}

	all := make([]*replica, 0, len(replicaIDs))
	for _, id := range replicaIDs {
		rep := newReplica(id, cfg, r)
		r.replicas[id] = rep
		all = append(all, rep)
	}
	r.lb = newRoundRobinBalancer(all)

	return r
}

func (r *router) getDataSet() *model.Dataset {
	return r.dataset
}

func (r *router) getReplica(i *model.Invocation) *replica {
	return r.replicas[i.GetReplicaID()]
}

// pickHedgeReplica returns an alternate replica for a hedge copy, chosen by
// the router's load-balancer policy, never the one the original request is
// already running on.
func (r *router) pickHedgeReplica(exclude string) *replica {
	return r.lb.pick(exclude)
}

func (r *router) forward(i *model.Invocation) {
	r.getReplica(i).forward(i)
}

func (r *router) terminate() {
	for _, rep := range r.replicas {
		rep.terminate()
	}
}

func (r *router) registerReplicaScaling(replicaID string, amount int64, timestamp float64) {
	r.replicasCount += amount
	replicasCountStr := strconv.FormatInt(r.replicasCount, 10)
	timestampStr := strconv.FormatFloat(timestamp, 'f', -1, 64)
	r.register = append(r.register, []string{replicaID, replicasCountStr, timestampStr})
}

func (r *router) GetOutPut() ([][]string, [][]string) {
	t_res := [][]string{}
	header := []string{"threadID", "replicaID", "busyTime", "upTime", "reqsProcessed", "lastWorkTS", "startTS", "shutdownTS"}
	t_res = append(t_res, header)
	for _, rep := range r.replicas {
		t_res = append(t_res, rep.getOutPut()...)
	}

	r_res := [][]string{}
	header = []string{"replicaID", "thread_amount", "timestamp"}
	r_res = append(r_res, header)
	r_res = append(r_res, r.register...)

	return t_res, r_res
}
