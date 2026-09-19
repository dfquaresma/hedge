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
	TracePath        string
	OutputPath       string
	Columns          model.ColumnMapping
	Techniques       []string
	TailLatencyProbs []string
	ThresholdScopes  []string
	MaxThreads       []int // per replica; empty or containing 0 sweeps "unlimited"
	MinGroupSize     int
}

// runSpec is one point of the parameter grid.
type runSpec struct {
	prob       string
	technique  string
	scope      string
	maxThreads int
}

// expandRuns builds the parameter grid. The threshold scope only matters for
// techniques that hedge, so baseline runs once per prob x maxThreads instead
// of once per scope — its results are identical under any scope.
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
		for _, mt := range maxThreads {
			for _, t := range sc.Techniques {
				if t == "baseline" {
					specs = append(specs, runSpec{prob: p, technique: t, scope: model.ScopePerGroup, maxThreads: mt})
					continue
				}
				for _, s := range scopes {
					specs = append(specs, runSpec{prob: p, technique: t, scope: s, maxThreads: mt})
				}
			}
		}
	}
	return specs
}

// Sim parses the trace once and replays it under every combination of
// tailLatencyProb x maxThreads x technique, writing one result set per run.
func Sim(sc SimConfig) {
	start := time.Now()

	trace, err := model.ParseTrace(sc.TracePath, sc.Columns, sc.MinGroupSize)
	if err != nil {
		panic(err)
	}

	// Written once per trace, not once per grid point: every run's
	// *-invocations.csv joins back to this file by rowID instead of
	// repeating identity columns that never change between runs.
	indexWriter, err := io.NewStreamWriter(sc.OutputPath, "trace-index.csv")
	if err != nil {
		panic(err)
	}
	if err := trace.WriteIndex(indexWriter); err != nil {
		panic(err)
	}
	if err := indexWriter.Close(); err != nil {
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
		MaxThreads:      spec.maxThreads,
		TailLatencyProb: spec.prob,
		Technique:       spec.technique,
	}

	maxThreadsDesc := "INF"
	if spec.maxThreads > 0 {
		maxThreadsDesc = fmt.Sprintf("%d", spec.maxThreads)
	}
	techDesc := spec.technique
	if spec.technique != "baseline" {
		techDesc = spec.technique + "_" + spec.scope
	}
	simulationName := fmt.Sprintf("%s_maxthreads%s_tlprob%s", techDesc, maxThreadsDesc, spec.prob)
	fmt.Printf("[%d/%d] Running %s -> %s\n", count, total, simulationName, sc.OutputPath)

	dataset := model.NewDataSet(trace, spec.prob, spec.scope)
	router := common.NewRouter(dataset, cfg, trace.ReplicaIDs)
	replayer := common.NewReplayer(dataset, router, simulationName)

	replayer.Run()
	fmt.Println("Simulation for " + simulationName + " is finished")

	invocationsWriter, err := io.NewStreamWriter(sc.OutputPath, simulationName+"-invocations.csv")
	if err != nil {
		panic(err)
	}
	if err := dataset.WriteOutput(invocationsWriter); err != nil {
		panic(err)
	}
	if err := invocationsWriter.Close(); err != nil {
		panic(err)
	}

	// threads/replicas outputs are small (hundreds to thousands of rows, one
	// row per thread or scaling event) — not the memory bottleneck the
	// invocations file is, so they stay buffered via WriteOutput.
	threadsOutput, scalingOutput := router.GetOutPut()
	io.WriteOutput(sc.OutputPath, simulationName+"-threads.csv", threadsOutput)
	io.WriteOutput(sc.OutputPath, simulationName+"-replicas.csv", scalingOutput)

	return replayer.GetOutPut()
}
