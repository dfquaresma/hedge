package model

import (
	"fmt"
	"sort"
	"strconv"
	"time"
)

// Trace is a parsed and enriched load-balancer trace. It is built once and
// reused across simulation runs; each run derives a fresh Dataset from it,
// since invocations are mutated during simulation.
type Trace struct {
	rows        []parsedRow
	percentiles map[string]percentile
	groupSizes  map[string]int64
	latencies   map[string][]float64
}

type parsedRow struct {
	appID    string
	funcID   string
	startTS  float64
	duration float64
}

type Dataset struct {
	iLen        int
	iterator    int
	invocations []Invocation
	latencies   map[string][]float64
}

// Timestamp layouts accepted for the start-timestamp column, tried in order.
// A plain float value is interpreted as epoch seconds.
var timestampLayouts = []string{
	time.RFC3339Nano,
	"2006-01-02 15:04:05.999999999",
	"2006-01-02 15:04:05",
}

// ParseTrace turns raw CSV records (header included) into a Trace. Columns
// are located by name through the ColumnMapping, so any CSV that carries the
// four required fields can be replayed regardless of column order or extra
// fields. Rows with a non-positive or unparsable duration and rows whose
// app+func group has fewer than minGroupSize samples are dropped: percentile
// estimates for tiny groups are meaningless as hedging thresholds.
// Timestamps are normalized to start at zero and rows are sorted by start
// time, as the replayer expects a chronological trace.
func ParseTrace(records [][]string, cols ColumnMapping, minGroupSize int) (*Trace, error) {
	if len(records) < 2 {
		return nil, fmt.Errorf("trace has no data rows")
	}

	colIdx, err := resolveColumns(records[0], cols)
	if err != nil {
		return nil, err
	}

	rows := make([]parsedRow, 0, len(records)-1)
	durationsByGroup := make(map[string][]float64)
	skipped := 0
	for _, record := range records[1:] {
		row, ok := parseRow(record, colIdx)
		if !ok {
			skipped++
			continue
		}
		rows = append(rows, row)
		key := row.appID + row.funcID
		durationsByGroup[key] = append(durationsByGroup[key], row.duration)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no valid rows in trace (%d skipped)", skipped)
	}

	percentiles := make(map[string]percentile)
	groupSizes := make(map[string]int64)
	for key, durations := range durationsByGroup {
		if len(durations) < minGroupSize {
			delete(durationsByGroup, key)
			continue
		}
		sort.Float64s(durations)
		percentiles[key] = percentile{
			p50:   quantile(durations, 0.50),
			p95:   quantile(durations, 0.95),
			p99:   quantile(durations, 0.99),
			p999:  quantile(durations, 0.999),
			p9999: quantile(durations, 0.9999),
			p100:  durations[len(durations)-1],
		}
		groupSizes[key] = int64(len(durations))
	}

	kept := rows[:0]
	for _, row := range rows {
		if _, ok := groupSizes[row.appID+row.funcID]; ok {
			kept = append(kept, row)
		}
	}
	rows = kept
	if len(rows) == 0 {
		return nil, fmt.Errorf("every app+func group has fewer than %d samples", minGroupSize)
	}

	sort.SliceStable(rows, func(i, j int) bool { return rows[i].startTS < rows[j].startTS })
	base := rows[0].startTS
	for i := range rows {
		rows[i].startTS -= base
	}

	fmt.Printf(
		"Trace parsed: %d invocations, %d app+func groups, %d rows dropped (invalid or below minGroupSize=%d)\n",
		len(rows), len(groupSizes), len(records)-1-len(rows), minGroupSize,
	)

	return &Trace{
		rows:        rows,
		percentiles: percentiles,
		groupSizes:  groupSizes,
		latencies:   durationsByGroup,
	}, nil
}

func resolveColumns(header []string, cols ColumnMapping) (map[string]int, error) {
	wanted := map[string]string{
		"app":            cols.App,
		"func":           cols.Func,
		"startTimestamp": cols.StartTimestamp,
		"duration":       cols.Duration,
	}
	colIdx := make(map[string]int, len(wanted))
	for field, name := range wanted {
		if name == "" {
			return nil, fmt.Errorf("column mapping for %q is empty", field)
		}
		found := -1
		for i, h := range header {
			if h == name {
				found = i
				break
			}
		}
		if found < 0 {
			return nil, fmt.Errorf("column %q (mapped to %s) not found in trace header %v", name, field, header)
		}
		colIdx[field] = found
	}
	return colIdx, nil
}

func parseRow(record []string, colIdx map[string]int) (parsedRow, bool) {
	for _, idx := range colIdx {
		if idx >= len(record) {
			return parsedRow{}, false
		}
	}
	appID := record[colIdx["app"]]
	funcID := record[colIdx["func"]]
	if appID == "" || funcID == "" {
		return parsedRow{}, false
	}
	duration, err := strconv.ParseFloat(record[colIdx["duration"]], 64)
	if err != nil || duration <= 0 {
		return parsedRow{}, false
	}
	startTS, err := parseTimestamp(record[colIdx["startTimestamp"]])
	if err != nil {
		return parsedRow{}, false
	}
	return parsedRow{appID: appID, funcID: funcID, startTS: startTS, duration: duration}, true
}

func parseTimestamp(s string) (float64, error) {
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return v, nil
	}
	for _, layout := range timestampLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return float64(t.UnixNano()) / 1e9, nil
		}
	}
	return 0, fmt.Errorf("unparsable timestamp: %q", s)
}

// quantile returns the nearest-rank quantile of an ascending-sorted slice.
func quantile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(p*float64(len(sorted))+0.5) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// NewDataSet materializes fresh invocations from a parsed trace for one
// simulation run, tagging each with the tail-latency threshold implied by
// tlProb (e.g. "p95", "p99").
func NewDataSet(t *Trace, tlProb string) *Dataset {
	invocs := make([]Invocation, len(t.rows))
	tailLatencyCount := 0
	for id, row := range t.rows {
		key := row.appID + row.funcID
		entry := traceEntry{
			appID:       row.appID,
			funcID:      row.funcID,
			groupSize:   t.groupSizes[key],
			startTS:     row.startTS,
			duration:    row.duration,
			endTS:       row.startTS + row.duration,
			tailLatency: newTailLatency(t.percentiles[key], tlProb),
		}
		if entry.duration > entry.tailLatency.getTailLatencyThreshold() {
			tailLatencyCount++
		}
		invocs[id] = *newInvocation(strconv.Itoa(id), entry)
	}

	fmt.Printf(
		"Number of Invocations: %d\nNumber of Tail Latency Reqs: %d\nPercentage Free of Tail Latency: %f\n\n",
		len(invocs),
		tailLatencyCount,
		1-(float64(tailLatencyCount)/float64(len(invocs))),
	)

	return &Dataset{
		iLen:        len(invocs),
		invocations: invocs,
		latencies:   t.latencies,
	}
}

func (d *Dataset) GetLatenciesOf(id string) []float64 {
	return d.latencies[id]
}

func (d *Dataset) Next() *Invocation {
	if !d.HasNext() {
		return nil
	}
	index := d.iterator
	d.iterator++
	return &d.invocations[index]
}

func (d *Dataset) HasNext() bool {
	return d.iterator < d.iLen
}

func (d *Dataset) GetSize() int {
	return len(d.invocations)
}

func (d *Dataset) GetOutPut() [][]string {
	res := [][]string{}
	header := []string{
		"appID", "funcID", "invocationID",
		"endTS", "startTS", "tl_threshold",
		"duration", "responseTime", "techniqueResponseTime",
	}
	res = append(res, header)
	for _, inv := range d.invocations {
		res = append(res, inv.getOutPut())
	}
	return res
}
