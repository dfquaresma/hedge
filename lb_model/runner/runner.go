package runner

import (
	"fmt"
	"time"

	"github.com/dfquaresma/hedge/common/io"
	"github.com/dfquaresma/hedge/lb_model/common"
	"github.com/dfquaresma/hedge/lb_model/model"
)

// SimConfig describes one trace section of config.json: which trace to
// replay, how to interpret its columns and the grid of simulation parameters
// to sweep.
type SimConfig struct {
	TracePath         string
	OutputPath        string
	Columns           model.ColumnMapping
	Techniques        []string
	TailLatencyProbs  []string
	ThresholdScopes   []string
	Idletimes         []int
	MaxThreads        []int // per replica; empty or containing 0 sweeps "unlimited"
	ForwardLatency    float64
	ColdStartDuration float64
	MinGroupSize      int
}

// runSpec is one point of the parameter grid.
type runSpec struct {
	prob       string
	technique  string
	scope      string
	idletime   float64
	maxThreads int
}

// expandRuns builds the parameter grid. The threshold scope only matters for
// techniques that hedge, so baseline runs once per prob x idletime x
// maxThreads instead of once per scope — its results are identical under any
// scope.
func expandRuns(sc SimConfig) []runSpec {
	scopes := sc.ThresholdScopes
	if len(scopes) == 0 {
		scopes = []string{model.ScopePerGroup}
	}
	maxThreads := sc.MaxThreads
	if len(maxThreads) == 0 {
		maxThreads = []int{0} // 0 = unlimited
	}
	specs := []runSpec{}
	for _, p := range sc.TailLatencyProbs {
		for _, i := range sc.Idletimes {
			for _, mt := range maxThreads {
				for _, t := range sc.Techniques {
					if t == "baseline" {
						specs = append(specs, runSpec{prob: p, technique: t, scope: model.ScopePerGroup, idletime: float64(i), maxThreads: mt})
						continue
					}
					for _, s := range scopes {
						specs = append(specs, runSpec{prob: p, technique: t, scope: s, idletime: float64(i), maxThreads: mt})
					}
				}
			}
		}
	}
	return specs
}

// Sim parses the trace once and replays it under every combination of
// tailLatencyProb x idletime x maxThreads x technique, writing one result
// set per run.
func Sim(sc SimConfig) {
	start := time.Now()

	trace, err := model.ParseTrace(sc.TracePath, sc.Columns, sc.MinGroupSize)
	if err != nil {
		panic(err)
	}

	specs := expandRuns(sc)
	io.WriteOutputHeaderRow(sc.OutputPath, "replayer-stats.csv", []string{"elapsedTime", "currentTime", "id"})
	for count, spec := range specs {
		replayerOut := simulate(trace, sc, spec, count+1, len(specs))
		io.WriteOutputByRow(
			sc.OutputPath,
			"replayer-stats.csv",
			[]string{
				replayerOut[0],
				time.Now().Format("2006-01-02 15:04:05"),
				replayerOut[1],
			},
		)
	}
	fmt.Printf("Total Simulation Time: %s\n", time.Since(start))
}

func simulate(trace *model.Trace, sc SimConfig, spec runSpec, count, total int) []string {
	cfg := model.Config{
		ForwardLatency:    sc.ForwardLatency,
		Idletime:          spec.idletime,
		ColdStartDuration: sc.ColdStartDuration,
		MaxThreads:        spec.maxThreads,
		TailLatencyProb:   spec.prob,
		Technique:         spec.technique,
	}

	idleDesc := "INF"
	if spec.idletime >= 0 {
		idleDesc = fmt.Sprintf("%.1f", spec.idletime)
	}
	maxThreadsDesc := "INF"
	if spec.maxThreads > 0 {
		maxThreadsDesc = fmt.Sprintf("%d", spec.maxThreads)
	}
	techDesc := spec.technique
	if spec.technique != "baseline" {
		techDesc = spec.technique + "_" + spec.scope
	}
	simulationName := fmt.Sprintf("%s_idletime%s_maxthreads%s_tlprob%s", techDesc, idleDesc, maxThreadsDesc, spec.prob)
	fmt.Printf("[%d/%d] Running %s -> %s\n", count, total, simulationName, sc.OutputPath)

	dataset := model.NewDataSet(trace, spec.prob, spec.scope)
	router := common.NewRouter(dataset, cfg)
	replayer := common.NewReplayer(dataset, router, simulationName)

	replayer.Run()
	fmt.Println("Simulation for " + simulationName + " is finished")

	io.WriteOutput(sc.OutputPath, simulationName+"-invocations.csv", dataset.GetOutPut())

	threadsOutput, scalingOutput := router.GetOutPut()
	io.WriteOutput(sc.OutputPath, simulationName+"-threads.csv", threadsOutput)
	io.WriteOutput(sc.OutputPath, simulationName+"-replicas.csv", scalingOutput)

	return replayer.GetOutPut()
}
