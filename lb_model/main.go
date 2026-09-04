package main

import (
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/spf13/viper"

	"github.com/dfquaresma/hedge/lb_model/model"
	"github.com/dfquaresma/hedge/lb_model/runner"
)

func main() {
	viper.SetConfigFile("config.json")
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Failed to read config file: %s", err)
	}

	sections := make([]string, 0)
	for section := range viper.AllSettings() {
		sections = append(sections, section)
	}
	sort.Strings(sections)

	for _, section := range sections {
		start := time.Now()
		fmt.Printf("=== Trace section: %s ===\n", section)
		runner.Sim(getConfig(section))
		fmt.Printf("Section %s TotalTime: %s\n\n", section, time.Since(start))
	}
}

func getConfig(s string) runner.SimConfig {
	return runner.SimConfig{
		TracePath:  viper.GetString(s + ".tracePath"),
		OutputPath: viper.GetString(s + ".outputPath"),
		Columns: model.ColumnMapping{
			App:            viper.GetString(s + ".columns.app"),
			Func:           viper.GetString(s + ".columns.func"),
			StartTimestamp: viper.GetString(s + ".columns.startTimestamp"),
			Duration:       viper.GetString(s + ".columns.duration"),
		},
		Techniques:        viper.GetStringSlice(s + ".resourceProvisioner.technique"),
		TailLatencyProbs:  viper.GetStringSlice(s + ".resourceProvisioner.tailLatencyProb"),
		Idletimes:         viper.GetIntSlice(s + ".resourceProvisioner.idletime"),
		ForwardLatency:    viper.GetFloat64(s + ".forwardLatency"),
		ColdStartDuration: viper.GetFloat64(s + ".coldStartDuration"),
		MinGroupSize:      viper.GetInt(s + ".minGroupSize"),
	}
}
