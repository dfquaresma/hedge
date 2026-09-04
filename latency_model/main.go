package main

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/spf13/viper"

	"github.com/dfquaresma/hedge/common/distuv"
	"github.com/dfquaresma/hedge/common/io"
)

func main() {
	firstStart := time.Now()
	viper.SetConfigFile("config.json")
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Failed to read config file: %s", err)
		return
	}

	requests_count := viper.GetInt("requests_count")
	outputPath := viper.GetString("outputPath")
	functions := viper.GetStringSlice("functions")

	sim_results := [][]string{
		{
			"technique",
			"app",
			"func",
			"end_timestamp",
			"response_time",
			"system_load",
			"service_time",
			"copy_service_time",
			"delay",
		},
	}
	viper.SetConfigFile("../synthetic_functions.json")
	err = viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Failed to read config file: %s", err)
		return
	}
	for _, f := range functions {
		start := time.Now()
		fmt.Printf("Running for function %s...", f)
		interarrival_distname := viper.GetString(f + ".interarrival_distribution")
		servicetime_distname := viper.GetString(f + ".servicetime_distribution")
		sim_results = append(
			sim_results,
			generate(requests_count, f, interarrival_distname, servicetime_distname)...,
		)
		fmt.Printf(" Finished. Time Running: %s\n", time.Since(start))
	}
	io.WriteOutput(outputPath, "latency_model-results.csv", sim_results)
	fmt.Printf("\nTotal Time of Simulation: %s\n", time.Since(firstStart))
}

func generate(requests_count int, f, interarrival_distname, servicetime_distname string) [][]string {
	interarrival_dist := distuv.NewDistribution(f, interarrival_distname)
	servicetime_dist := distuv.NewDistribution(f, servicetime_distname)
	if interarrival_dist == nil || servicetime_dist == nil {
		panic(fmt.Sprintf("Either %s or %s for %s is not valid", interarrival_distname, servicetime_distname, f))
	}
	traffic := newTraffic(f, interarrival_dist, servicetime_dist)

	ts := 0.0
	generated_app := servicetime_distname + "_" + interarrival_distname + "-app"
	generated_func := f
	workload := [][]string{}
	for i := 0; i < requests_count; i++ {
		ts += traffic.NextArrivalValue()
		service_time := traffic.NextServiceValue()
		copy_service_time := traffic.NextServiceValue()

		workload = append(
			workload,
			getBaseline(service_time, ts, generated_app, generated_func),
			newHedge("naive_hedge", servicetime_dist.GetPercentile(0.95)).hedgedRequest(
				generated_app, generated_func, ts, service_time, copy_service_time,
			),
			newHedge("delayed_hedge_p95wc", servicetime_dist.GetPercentile(0.95)).hedgedRequest(
				generated_app, generated_func, ts, service_time, copy_service_time,
			),
			newHedge("perfect_hedge", servicetime_dist.GetPercentile(0.95)).hedgedRequest(
				generated_app, generated_func, ts, service_time, copy_service_time,
			),
		)
	}
	return workload
}

func getBaseline(service_time, ts float64, generated_app, generated_func string) []string {
	response_time := service_time
	end_timestamp := ts + response_time
	total_time_running_functions := response_time // vanilla scenario, send just one and that's all
	return []string{
		"baseline",
		generated_app,
		generated_func,
		strconv.FormatFloat(end_timestamp, 'f', -1, 64),
		strconv.FormatFloat(response_time, 'f', -1, 64),
		strconv.FormatFloat(total_time_running_functions, 'f', -1, 64),
		strconv.FormatFloat(service_time, 'f', -1, 64),
		"0",
		"0",
	}
}
