package main

import (
	"flag"
	"fmt"
	"log"
    "math/rand"
	"os"
	"strconv"
	"strings"
	"time"
	"github.com/robvanmieghem/go-opencl/cl"
)

//Version is the released version string of gominer
const Version = "0.0.0"

// pool Client
func submitSolutions(pool HeaderReporter, solutionChannel chan []byte) {
	for header := range solutionChannel {
		if err := pool.SubmitHeader(header); err != nil {
			log.Println("Error submitting solution -", err)
		}
		log.Println("Submitted header:", header)
	}
}

func main() {
	printVersion := flag.Bool("v", false, "Show version and exit")
    host := flag.String("p", "us-east1.ethereum.miningpoolhub.com:20536", "pool host:port")
//	secondsOfWorkPerRequestedHeader := flag.Int("S", 10, "Time between calls to pool")
	excludedGPUs := flag.String("e", "", "exclude GPUs: comma separated list of devicenumbers")
	queryString := flag.String("a", "0xeb9310b185455f863f526dab3d245809f6854b4d", "eth address or pool account")
	flag.Parse()

	if *printVersion {
		fmt.Println("minr version", Version)
		os.Exit(0)
	}

    rand.Seed(time.Now().UnixNano())

	platforms, err := cl.GetPlatforms()
	if len(platforms) == 0 {
		log.Println("No OpenCL Platforms found.", err)
		os.Exit(1)
	}

	clDevices := make([]*cl.Device)
	for _, platform := range platforms {
		platormDevices, err := cl.GetDevices(platform, cl.DeviceTypeGPU)
		if err != nil {
			log.Println(err)
		}
		for i, device := range platormDevices {
			log.Println("GPU", i, device.Name(), device.MaxComputeUnits())
			clDevices = append(clDevices, device)
		}
	}

	numDevices := len(clDevices)
	if numDevices == 0 {
		log.Println("No opencl GPU devices found")
		os.Exit(1)
	}

    // a channel buffer size of 2 is probably enough, but use more
	solutionChannel := make(chan []byte, numDevices)
	go submitSolutions(pool, solutionChannel)

	//Start mining routines
	var hashRateReportsChannel = make(chan *HashRateReport, numDevices*4)
	miners := make([]*Miner)
	globalItemSize := 32768
	for i, device := range clDevices {
		if deviceExcludedForMining(i, *excludedGPUs) {
			continue
		}
		workChannel := make(chan *MiningWork, 1)
		workChannels = append(workChannels, workChannel)
		miner := &Miner{
			clDevice:          device,
			minerID:           i,
			hashRateReports:   hashRateReportsChannel,
			miningWorkChannel: workChannel,
			solutionChannel:   solutionChannel,
			GlobalItemSize:    globalItemSize,
		}
        append(miners, miner)
		go miner.mine()
	}

	pool := &Client{*host, *queryString, miners}

	//Start printing out the hashrates of the different gpu's
	hashRateReports := make([]float64, numDevices)
	for {
		//No need to print at every hashreport, we have time
		for i := 0; i < numDevices; i++ {
			report := <-hashRateReportsChannel
			hashRateReports[report.MinerID] = report.HashRate
		}
		fmt.Print("\r")
		var totalHashRate float64
		for minerID, hashrate := range hashRateReports {
			fmt.Printf("%d-%.1f ", minerID, hashrate)
			totalHashRate += hashrate
		}
		fmt.Printf("Total: %.1f MH/s  ", totalHashRate)

	}
}

//deviceExcludedForMining checks if the device is in the exclusion list
func deviceExcludedForMining(deviceID int, excludedGPUs string) bool {
	excludedGPUList := strings.Split(excludedGPUs, ",")
	for _, excludedGPU := range excludedGPUList {
		if strconv.Itoa(deviceID) == excludedGPU {
			return true
		}
	}
	return false
}
