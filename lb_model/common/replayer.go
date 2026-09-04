package common

import (
	"fmt"
	"time"

	"github.com/agoussia/godes"
	"github.com/dfquaresma/hedge/lb_model/model"
)

const progressLogInterval = 100000

type replayer struct {
	*godes.Runner
	dataset     *model.Dataset
	router      *router
	id          string
	elapsedTime time.Duration
}

func NewReplayer(dataset *model.Dataset, router *router, id string) *replayer {
	return &replayer{
		Runner:  &godes.Runner{},
		dataset: dataset,
		router:  router,
		id:      id,
	}
}

func (re *replayer) Run() {
	start := time.Now()
	fmt.Println("Starting Replayer...")
	godes.Run()

	size := re.dataset.GetSize()
	progress := 0
	previousTs := 0.0
	for i := re.dataset.Next(); i != nil; i = re.dataset.Next() {
		currStartTs := i.GetStartTS()
		godes.Advance(currStartTs - previousTs)
		previousTs = currStartTs
		i.SetForwardedTs(godes.GetSystemTime())
		re.router.forward(i)
		progress++
		if progress%progressLogInterval == 0 {
			fmt.Printf("[%s] %d/%d invocations replayed\n", re.id, progress, size)
		}
	}
	re.router.terminate()
	godes.WaitUntilDone()
	godes.Clear()

	re.elapsedTime = time.Since(start)
}

func (re *replayer) GetOutPut() []string {
	return []string{re.elapsedTime.String(), re.id}
}
