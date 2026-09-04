package io

import (
	"encoding/csv"
	"os"
)

func ReadInput(tracePath string) [][]string {
	return ReadCSV(tracePath)[1:]
}

// ReadCSV reads a CSV file keeping the header row, for callers that resolve
// columns by name.
func ReadCSV(tracePath string) [][]string {
	input, err := os.Open(tracePath)
	if err != nil {
		panic(err)
	}
	defer input.Close()

	r := csv.NewReader(input)
	rows, err := r.ReadAll()
	if err != nil {
		panic(err)
	}
	return rows
}
