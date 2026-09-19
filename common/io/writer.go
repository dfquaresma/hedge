package io

import (
	"encoding/csv"
	"os"
)

func WriteOutput(outputPath, simulationName string, data [][]string) {
	err := os.MkdirAll(outputPath, os.ModePerm)
	if err != nil {
		panic(err)
	}

	filePath := outputPath + simulationName
	output, err := os.Create(filePath)
	if err != nil {
		panic(err)
	}
	defer output.Close()

	writer := csv.NewWriter(output)
	defer writer.Flush()

	for _, record := range data {
		err := writer.Write(record)
		if err != nil {
			panic(err)
		}
	}
}

// StreamWriter writes many records to one CSV file, opening it once and
// flushing on Close. Use it instead of WriteOutput for large outputs (millions
// of rows) that would otherwise need to be buffered entirely in memory as a
// [][]string before a single write.
type StreamWriter struct {
	file   *os.File
	writer *csv.Writer
}

func NewStreamWriter(outputPath, simulationName string) (*StreamWriter, error) {
	if err := os.MkdirAll(outputPath, os.ModePerm); err != nil {
		return nil, err
	}
	f, err := os.Create(outputPath + simulationName)
	if err != nil {
		return nil, err
	}
	return &StreamWriter{file: f, writer: csv.NewWriter(f)}, nil
}

func (w *StreamWriter) Write(record []string) error {
	return w.writer.Write(record)
}

// Close flushes buffered writes and closes the underlying file. It must be
// called for the CSV writer's internal buffer to actually reach disk.
func (w *StreamWriter) Close() error {
	w.writer.Flush()
	if err := w.writer.Error(); err != nil {
		w.file.Close()
		return err
	}
	return w.file.Close()
}

func WriteOutputHeaderRow(outputPath, simulationName string, rows []string) {
	_ = os.Remove(outputPath + simulationName)
	WriteOutputByRow(outputPath, simulationName, rows)
}

func WriteOutputByRow(outputPath, simulationName string, rows []string) {
	err := os.MkdirAll(outputPath, os.ModePerm)
	if err != nil {
		panic(err)
	}

	filePath := outputPath + simulationName
	// Open the file in append mode, create if not exists
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	err = writer.Write(rows)
	if err != nil {
		panic(err)
	}
}
