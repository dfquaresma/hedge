package common

import (
	"strconv"

	"github.com/dfquaresma/hedge/lb_model/model"
)

type router struct {
	provisioners  map[string]*provisioner
	dataset       *model.Dataset
	cfg           model.Config
	register      [][]string
	replicasCount int64
}

func NewRouter(dataset *model.Dataset, cfg model.Config) *router {
	return &router{
		provisioners: make(map[string]*provisioner),
		dataset:      dataset,
		cfg:          cfg,
		register:     [][]string{},
	}
}

func (r *router) getDataSet() *model.Dataset {
	return r.dataset
}

func (r *router) getProvisioner(i *model.Invocation) *provisioner {
	rp := r.provisioners[i.GetAppID()+i.GetFuncID()]
	if rp == nil {
		rp = r.newProvisioner(i.GetAppID(), i.GetFuncID())
	}
	return rp
}

func (r *router) newProvisioner(aid, fid string) *provisioner {
	rp := newProvisioner(aid, fid, r.cfg, r)
	r.provisioners[aid+fid] = rp
	return rp
}

func (r *router) forward(i *model.Invocation) {
	r.getProvisioner(i).forward(i)
}

func (r *router) terminate() {
	for _, p := range r.provisioners {
		p.terminate()
	}
}

func (r *router) registerReplicaScaling(funcID string, amount int64, timestamp float64) {
	r.replicasCount += amount
	replicasCountStr := strconv.FormatInt(r.replicasCount, 10)
	timestampStr := strconv.FormatFloat(timestamp, 'f', -1, 64)
	r.register = append(r.register, []string{funcID, replicasCountStr, timestampStr})
}

func (r *router) GetOutPut() ([][]string, [][]string) {
	p_res := [][]string{}
	header := []string{"replicaID", "rpID", "appID", "funcID", "busyTime", "upTime", "reqsProcessed", "lastWorkTS", "startTS", "shutdownTS"}
	p_res = append(p_res, header)
	for _, p := range r.provisioners {
		p_res = append(p_res, p.getOutPut()...)
	}

	r_res := [][]string{}
	header = []string{"funcID", "replica_amount", "timestamp"}
	r_res = append(r_res, header)
	r_res = append(r_res, r.register...)

	return p_res, r_res
}
