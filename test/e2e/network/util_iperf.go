/*
Copyright 2015 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package network

// Tests network performance using iperf or other containers.
import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	megabyte = 1024 * 1024
)

// IPerfResults is a struct that stores some IPerfCSVResult
type IPerfResults struct {
	BandwidthMap map[string]int64
}

// IPerfCSVResult struct modelling an iperf record....
// 20160314154239,172.17.0.3,34152,172.17.0.2,5001,3,0.0-10.0,33843707904,27074774092
type IPerfCSVResult struct {
	date          string // field 1 in the csv
	cli           string // field 2 in the csv
	cliPort       int64  // ...
	server        string
	servPort      int64
	id            string
	interval      string
	transferBits  int64
	bandwidthBits int64
}

func (i *IPerfCSVResult) bandwidthMB() int64 {
	return int64(math.Round(float64(i.bandwidthBits) / float64(megabyte) / 8))
}

// Add adds a new result to the Results struct.
func (i *IPerfResults) Add(ipr *IPerfCSVResult) {
	if i.BandwidthMap == nil {
		i.BandwidthMap = map[string]int64{}
	}
	i.BandwidthMap[ipr.cli] = ipr.bandwidthBits
}

// ToTSV exports an easily readable tab delimited format of all IPerfResults.
func (i *IPerfResults) ToTSV() string {
	if len(i.BandwidthMap) < 1 {
		framework.Logf("Warning: no data in bandwidth map")
	}

	var buffer bytes.Buffer
	for node, bandwidth := range i.BandwidthMap {
		asJSON, _ := json.Marshal(node)
		buffer.WriteString("\t " + string(asJSON) + "\t " + fmt.Sprintf("%E", float64(bandwidth)))
	}
	return buffer.String()
}

// NewIPerf parses an IPerf CSV output line into an IPerfCSVResult.
func NewIPerf(csvLine string) (*IPerfCSVResult, error) {
	if len(csvLine) == 0 {
		return nil, fmt.Errorf("no iperf output received in csv line")
	}
	csvLine = strings.Trim(csvLine, "\n")
	slice := StrSlice(strings.Split(csvLine, ","))
	// iperf 2.19+ reports 15 fields, before it was just 9
	if len(slice) != 15 {
		return nil, fmt.Errorf("incorrect fields in the output: %v (%v out of 15)", slice, len(slice))
	}
	i := IPerfCSVResult{}
	i.date = slice.get(0)
	i.cli = slice.get(1)
	i.cliPort = intOrFail("client port", slice.get(2))
	i.server = slice.get(3)
	i.servPort = intOrFail("server port", slice.get(4))
	i.id = slice.get(5)
	i.interval = slice.get(6)
	i.transferBits = intOrFail("transfer port", slice.get(7))
	i.bandwidthBits = intOrFail("bandwidth port", slice.get(8))
	return &i, nil
}

// StrSlice represents a string slice
type StrSlice []string

func (s StrSlice) get(i int) string {
	if i >= 0 && i < len(s) {
		return s[i]
	}
	return ""
}

// intOrFail is a convenience function for parsing integers.
func intOrFail(debugName string, rawValue string) int64 {
	value, err := strconv.ParseInt(rawValue, 10, 64)
	if err != nil {
		framework.Failf("Failed parsing value %v from the string '%v' as an integer", debugName, rawValue)
	}
	return value
}

// IPerf2EnhancedCSVResults models the results produced by iperf2 when run with the -e (--enhancedreports) flag.
type IPerf2EnhancedCSVResults struct {
	Intervals []*IPerfCSVResult
	Total     *IPerfCSVResult
}

// ParseIPerf2EnhancedResultsFromCSV parses results from iperf2 when given the -e (--enhancedreports)
// and `--reportstyle C` options.
// Example output for version < 2.19 (agnhost < 2.53):
// 20201210141800.884,10.244.2.24,47880,10.96.114.79,6789,3,0.0-1.0,1677852672,13422821376
// 20201210141801.881,10.244.2.24,47880,10.96.114.79,6789,3,1.0-2.0,1980760064,15846080512
// 20201210141802.883,10.244.2.24,47880,10.96.114.79,6789,3,2.0-3.0,1886650368,15093202944
// Example output with version >= 2.19 (agnhost >= 2.53)
// +0000:20240908113035.128,192.168.9.3,58256,192.168.9.4,5001,1,0.0-1.0,5220466748,41763733984,-1,-1,-1,-1,0,0
// +0000:20240908113036.128,192.168.9.3,58256,192.168.9.4,5001,1,1.0-2.0,5127667712,41021341696,-1,-1,-1,-1,0,0
// +0000:20240908113037.128,192.168.9.3,58256,192.168.9.4,5001,1,2.0-3.0,5127405568,41019244544,-1,-1,-1,-1,0,0
// +0000:20240908113038.128,192.168.9.3,58256,192.168.9.4,5001,1,3.0-4.0,5173018624,41384148992,-1,-1,-1,-1,0,0
// +0000:20240908113039.128,192.168.9.3,58256,192.168.9.4,5001,1,4.0-5.0,5245894656,41967157248,-1,-1,-1,-1,0,0
// +0000:20240908113040.128,192.168.9.3,58256,192.168.9.4,5001,1,5.0-6.0,5213257728,41706061824,-1,-1,-1,-1,0,0
// +0000:20240908113041.128,192.168.9.3,58256,192.168.9.4,5001,1,6.0-7.0,5113118720,40904949760,-1,-1,-1,-1,0,0
// +0000:20240908113042.128,192.168.9.3,58256,192.168.9.4,5001,1,7.0-8.0,5242748928,41941991424,-1,-1,-1,-1,0,0
func ParseIPerf2EnhancedResultsFromCSV(output string) (*IPerf2EnhancedCSVResults, error) {
	var parsedResults []*IPerfCSVResult
	for _, line := range strings.Split(output, "\n") {
		parsed, err := NewIPerf(line)
		if err != nil {
			return nil, err
		}
		parsedResults = append(parsedResults, parsed)
	}
	if len(parsedResults) == 0 {
		return nil, fmt.Errorf("no results parsed from iperf2 output")
	}
	// format:
	// all but last lines are intervals
	intervals := parsedResults[:len(parsedResults)-1]
	// last line is an aggregation
	total := parsedResults[len(parsedResults)-1]
	return &IPerf2EnhancedCSVResults{
		Intervals: intervals,
		Total:     total,
	}, nil
}

// IPerf2NodeToNodeCSVResults models the results of running iperf2 between a daemonset of clients and
// a single server.  The node name of the server is captured, along with a map of client node name
// to iperf2 results.
type IPerf2NodeToNodeCSVResults struct {
	ServerNode string
	Results    map[string]*IPerf2EnhancedCSVResults
}
