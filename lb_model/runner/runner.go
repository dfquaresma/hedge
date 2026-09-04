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
	Idletimes         []int
	ForwardLatency    float64
	ColdStartDuration float64
	MinGroupSize      int
}

// Sim parses the trace once and replays it under every combination of
// tailLatencyProb x idletime x technique, writing one result set per run.
func Sim(sc SimConfig) {
	start := time.Now()

	records := io.ReadCSV(sc.TracePath)
	trace, err := model.ParseTrace(records, sc.Columns, sc.MinGroupSize)
	if err != nil {
		panic(err)
	}

	count := 1
	total := len(sc.Idletimes) * len(sc.TailLatencyProbs) * len(sc.Techniques)
	io.WriteOutputHeaderRow(sc.OutputPath, "replayer-stats.csv", []string{"elapsedTime", "currentTime", "id"})
	for _, p := range sc.TailLatencyProbs {
		for _, i := range sc.Idletimes {
			for _, t := range sc.Techniques {
				replayerOut := simulate(trace, sc, p, t, float64(i), count, total)
				io.WriteOutputByRow(
					sc.OutputPath,
					"replayer-stats.csv",
					[]string{
						replayerOut[0],
						time.Now().Format("2006-01-02 15:04:05"),
						replayerOut[1],
					},
				)
				count++
			}
		}
	}
	fmt.Printf("Total Simulation Time: %s\n", time.Since(start))
}

func simulate(trace *model.Trace, sc SimConfig, prob, technique string, idletime float64, count, total int) []string {
	cfg := model.Config{
		ForwardLatency:    sc.ForwardLatency,
		Idletime:          idletime,
		ColdStartDuration: sc.ColdStartDuration,
		TailLatencyProb:   prob,
		Technique:         technique,
	}

	idleDesc := "INF"
	if idletime >= 0 {
		idleDesc = fmt.Sprintf("%.1f", idletime)
	}
	simulationName := fmt.Sprintf("%s_idletime%s_tlprob%s", technique, idleDesc, prob)
	fmt.Printf("[%d/%d] Running %s -> %s\n", count, total, simulationName, sc.OutputPath)

	dataset := model.NewDataSet(trace, prob)
	router := common.NewRouter(dataset, cfg)
	replayer := common.NewReplayer(dataset, router, simulationName)

	replayer.Run()
	fmt.Println("Simulation for " + simulationName + " is finished")

	io.WriteOutput(sc.OutputPath, simulationName+"-invocations.csv", dataset.GetOutPut())

	replicasOutput, scalingOutput := router.GetOutPut()
	io.WriteOutput(sc.OutputPath, simulationName+"-replicas.csv", replicasOutput)
	io.WriteOutput(sc.OutputPath, simulationName+"-provisioners.csv", scalingOutput)

	return replayer.GetOutPut()
}
