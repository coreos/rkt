// Copyright 2016 The rkt Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"time"

	"github.com/appc/spec/schema"
	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/process"
	"github.com/spf13/cobra"
)

type ProcessStatus struct {
	Pid  int32
	Name string  // Name of process
	CPU  float64 // Percent of CPU used since last check
	VMS  uint64  // Virtual memory size
	RSS  uint64  // Resident set size
	Swap uint64  // Swap size
}

var (
	pidMap map[int32]*process.Process

	flagVerbose    bool
	flagDuration   string
	flagShowOutput bool
	saveToCsv      bool

	cmdRktMonitor = &cobra.Command{
		Use:     "rkt-monitor IMAGE",
		Short:   "Runs the specified ACI or pod manifest with rkt, and monitors rkt's usage",
		Example: "rkt-monitor mem-stresser.aci -v -d 30s",
		Run:     runRktMonitor,
	}
)

func init() {
	pidMap = make(map[int32]*process.Process)

	cmdRktMonitor.Flags().BoolVarP(&flagVerbose, "verbose", "v", false, "Print current usage every second")
	cmdRktMonitor.Flags().StringVarP(&flagDuration, "duration", "d", "10s", "How long to run the ACI")
	cmdRktMonitor.Flags().BoolVarP(&flagShowOutput, "show-output", "o", false, "Display rkt's stdout and stderr")
	cmdRktMonitor.Flags().BoolVarP(&saveToCsv, "to-file", "f", false, "Save benchmark results to benchmark.csv file")

	flag.Parse()
}

func main() {
	cmdRktMonitor.Execute()
}

func runRktMonitor(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		cmd.Usage()
		os.Exit(1)
	}

	d, err := time.ParseDuration(flagDuration)
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}

	if os.Getuid() != 0 {
		fmt.Printf("need to be root to run rkt images\n")
		os.Exit(1)
	}

	f, err := os.Open(args[0])
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
	decoder := json.NewDecoder(f)

	podManifest := false
	man := schema.PodManifest{}
	err = decoder.Decode(&man)
	if err == nil {
		podManifest = true
	}

	var execCmd *exec.Cmd
	if podManifest {
		execCmd = exec.Command("rkt", "run", "--pod-manifest", args[0], "--net=default")
	} else {
		execCmd = exec.Command("rkt", "run", args[0], "--insecure-options=image", "--net=default")
	}
	if flagShowOutput {
		execCmd.Stdout = os.Stdout
		execCmd.Stderr = os.Stderr
	}

	containerStarting := time.Now()

	err = execCmd.Start()
	if err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
	containerStarted := time.Now()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for range c {
			err := killAllChildren(int32(execCmd.Process.Pid))
			if err != nil {
				fmt.Fprintf(os.Stderr, "cleanup failed: %v\n", err)
			}
			os.Exit(1)
		}
	}()

	usages := make(map[int32][]*ProcessStatus)

	timeToStop := time.Now().Add(d)

	records := [][]string{{"Time", "PID name", "PID number", "RSS", "CPU"}} // csv headers

	for time.Now().Before(timeToStop) {
		usage, err := getUsage(int32(execCmd.Process.Pid))
		if err != nil {
			panic(err)
		}
		if flagVerbose {
			printUsage(usage)
		}

		if saveToCsv {
			records = addRecords(usage, records)
		}

		for _, ps := range usage {
			usages[ps.Pid] = append(usages[ps.Pid], ps)
		}

		_, err = process.NewProcess(int32(execCmd.Process.Pid))
		if err != nil {
			// process.Process.IsRunning is not implemented yet
			fmt.Fprintf(os.Stderr, "rkt exited prematurely\n")
			break
		}

		time.Sleep(time.Second)
	}

	loadAvg, err := load.Avg()

	containerStoping := time.Now()
	err = killAllChildren(int32(execCmd.Process.Pid))
	if err != nil {
		fmt.Fprintf(os.Stderr, "cleanup failed: %v\n", err)
	}
	containerStopped := time.Now()

	for _, processHistory := range usages {
		var avgCPU float64
		var avgMem uint64
		var peakMem uint64

		for _, p := range processHistory {
			avgCPU += p.CPU
			avgMem += p.RSS
			if peakMem < p.RSS {
				peakMem = p.RSS
			}
		}

		avgCPU = avgCPU / float64(len(processHistory))
		avgMem = avgMem / uint64(len(processHistory))

		fmt.Printf("%s(%d): seconds alive: %d  avg CPU: %f%%  avg Mem: %s  peak Mem: %s\n", processHistory[0].Name, processHistory[0].Pid, len(processHistory), avgCPU, formatSize(avgMem), formatSize(peakMem))
	}

	if saveToCsv {
		saveRecords(records)
	}

	fmt.Printf("load average: Load1: %f Load5: %f Load15: %f\n", loadAvg.Load1, loadAvg.Load5, loadAvg.Load15)

	fmt.Printf("container start time: %dns\n", containerStarted.Sub(containerStarting).Nanoseconds())
	fmt.Printf("container stop time: %dns\n", containerStopped.Sub(containerStoping).Nanoseconds())
}

func killAllChildren(pid int32) error {
	p, err := process.NewProcess(pid)
	if err != nil {
		return err
	}
	processes := []*process.Process{p}
	for i := 0; i < len(processes); i++ {
		children, err := processes[i].Children()
		if err != nil && err != process.ErrorNoChildren {
			return err
		}
		processes = append(processes, children...)
	}
	for _, p := range processes {
		osProcess, err := os.FindProcess(int(p.Pid))
		if err != nil {
			if err.Error() == "os: process already finished" {
				continue
			}
			return err
		}
		err = osProcess.Kill()
		if err != nil {
			return err
		}
	}
	return nil
}

func getUsage(pid int32) ([]*ProcessStatus, error) {
	var statuses []*ProcessStatus
	pids := []int32{pid}
	for i := 0; i < len(pids); i++ {
		proc, ok := pidMap[pids[i]]
		if !ok {
			var err error
			proc, err = process.NewProcess(pids[i])
			if err != nil {
				return nil, err
			}
			pidMap[pids[i]] = proc
		}
		s, err := getProcStatus(proc)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, s)

		children, err := proc.Children()
		if err != nil && err != process.ErrorNoChildren {
			return nil, err
		}

	childloop:
		for _, child := range children {
			for _, p := range pids {
				if p == child.Pid {
					fmt.Printf("%d is in %#v\n", p, pids)
					continue childloop
				}
			}
			pids = append(pids, child.Pid)
		}
	}
	return statuses, nil
}

func getProcStatus(p *process.Process) (*ProcessStatus, error) {
	n, err := p.Name()
	if err != nil {
		return nil, err
	}
	c, err := p.Percent(0)
	if err != nil {
		return nil, err
	}
	m, err := p.MemoryInfo()
	if err != nil {
		return nil, err
	}
	return &ProcessStatus{
		Pid:  p.Pid,
		Name: n,
		CPU:  c,
		VMS:  m.VMS,
		RSS:  m.RSS,
		Swap: m.Swap,
	}, nil
}

func formatSize(size uint64) string {
	if size > 1024*1024*1024 {
		return strconv.FormatUint(size/(1024*1024*1024), 10) + " gB"
	}
	if size > 1024*1024 {
		return strconv.FormatUint(size/(1024*1024), 10) + " mB"
	}
	if size > 1024 {
		return strconv.FormatUint(size/1024, 10) + " kB"
	}
	return strconv.FormatUint(size, 10) + " B"
}

func printUsage(statuses []*ProcessStatus) {
	for _, s := range statuses {
		fmt.Printf("%s(%d): Mem: %s CPU: %f\n", s.Name, s.Pid, formatSize(s.RSS), s.CPU)
	}
	fmt.Printf("\n")
}

func addRecords(statuses []*ProcessStatus, records [][]string) [][]string {
	for _, s := range statuses {
		records = append(records, []string{time.Now().String(), s.Name, strconv.Itoa(int(s.Pid)), formatSize(s.RSS), strconv.FormatFloat(s.CPU, 'g', 1, 64)})
	}
	return records
}

func saveRecords(records [][]string) error {
	csvFile, err := os.Create("benchmark.csv")
	defer csvFile.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Can't write to a file: %v\n", err)
		os.Exit(1)
	}

	w := csv.NewWriter(csvFile)
	w.WriteAll(records)
	if err := w.Error(); err != nil {
		return err
	}

	return nil
}
