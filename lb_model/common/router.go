package common

import (
	"strconv"

	"github.com/dfquaresma/hedge/lb_model/model"
)

// router dispatches each invocation to the replica identified by its
// replicaID alone — tenant is not part of the lookup key, since a physical
// replica is shared by every tenant routed to it (multi-tenant contention on
// shared infrastructure is exactly what this model is meant to capture).
type router struct {
	replicas      map[string]*replica
	dataset       *model.Dataset
	cfg           model.Config
	register      [][]string
	replicasCount int64
}

func NewRouter(dataset *model.Dataset, cfg model.Config) *router {
	return &router{
		replicas: make(map[string]*replica),
		dataset:  dataset,
		cfg:      cfg,
		register: [][]string{},
	}
}

func (r *router) getDataSet() *model.Dataset {
	return r.dataset
}

func (r *router) getReplica(i *model.Invocation) *replica {
	rep := r.replicas[i.GetReplicaID()]
	if rep == nil {
		rep = r.newReplica(i.GetReplicaID())
	}
	return rep
}

func (r *router) newReplica(replicaID string) *replica {
	rep := newReplica(replicaID, r.cfg, r)
	r.replicas[replicaID] = rep
	return rep
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
