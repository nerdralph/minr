package main

import (
    "bufio"
    "bytes"
	"encoding/binary"
    "encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
    "math"
    "math/rand"
	"net"
	"strconv"
	"strings"
	"time"
	"github.com/nerdralph/go-opencl/cl"
	"github.com/nerdralph/crypto/sha3"
)


func debug(fmt string, args ...interface{}) {
	log.Printf("dbg:" + fmt, args...)
}

const (
	Version = "0.0.0"
	Kernel= "eth.cl"
	MB				 = 1024 * 1024
	cacheBYTESINIT   = 16 * MB
	cacheBYTESGROWTH = 128 * 1024
	cacheROUNDS      = 3
	hashBYTES        = 64
)

func isPrime(n int32) bool {
	// if (n == 2) || (n == 3) { return true }
	if n%2 == 0 { return false }
	if n%3 == 0 { return false }
	sqrt := int32(math.Sqrt(float64(n)))
	for i := int32(5); i <= sqrt; i += 6 {
		if n%i == 0 { return false }
		if n%(i+2) == 0 { return false }
	}
	return true
}

func cacheSize(epoch int) int {
	sz := cacheBYTESINIT + cacheBYTESGROWTH*epoch
	sz -= hashBYTES
	for ; !isPrime(int32(sz / hashBYTES)); sz -= 2 * hashBYTES { }
	return sz
}

func makeCache(epoch int, seed []byte) []byte {
    sz := cacheSize(epoch)
    cache := make([]byte, sz)
	digestStart := sha3.SumK512(seed)
	kf512 := sha3.ReHashK512()
	digest := kf512.Data()
	copy(digest, digestStart[:])

    for pos := 0; pos < sz; pos += hashBYTES {
        copy(cache[pos:], digest)
		kf512.Hash()
    }

	// Use a low-round version of randmemohash
	rows := sz / hashBYTES
	for i := 0; i < cacheROUNDS; i++ {
		for j := 0; j < rows; j++ {
			var (
				srcOff = ((j - 1 + rows) % rows) * hashBYTES
				dstOff = j * hashBYTES
				xorOff = (binary.LittleEndian.Uint32(cache[dstOff:]) % uint32(rows)) * hashBYTES
			)
			sha3.FastXORWords(digest, cache[srcOff:srcOff+hashBYTES], cache[xorOff:xorOff+hashBYTES])
			kf512.Hash()
            copy(cache[dstOff:], digest)
		}
	}
	return cache
}

type jhdr struct {
	Id      int32  `json:"id"`
	Jsonrpc string `json:"jsonrpc"`
}
type jbody struct {
	Method string   `json:"method"`
	Params []string `json:"params"`
}
type jmsg struct {
	jhdr
	jbody
}

/*
// pool Client
func submitSolutions(pool HeaderReporter, solutionChannel chan []byte) {
	for header := range solutionChannel {
		if err := pool.SubmitHeader(header); err != nil {
			log.Println("Error submitting solution -", err)
		}
		log.Println("Submitted header:", header)
	}
}
*/

func main() {
	printVersion := flag.Bool("v", false, "Show version and print devices")
    //pool := flag.String("p", "us-east1.ethereum.miningpoolhub.com:20536", "pool host:port")
    pool := flag.String("p", "eth-us-east1.nanopool.org:9999", "pool host:port")
	addr := flag.String("a", "0xeb9310b185455f863f526dab3d245809f6854b4d", "eth address or pool account")
/*
	excludedGPUs := flag.String("e", "", "exclude GPUs: comma separated list of devicenumbers")
*/
	flag.Parse()

	log.Println("minr version", Version)

	context, _ := cl.CreateContextFromType(cl.DeviceTypeGPU)
	devices := context.Devices
	numDevices := len(devices)
	fmt.Println("Found", numDevices, "GPU devices:")

	var queues []*cl.CommandQueue
	for i, device := range devices {
		fmt.Println("GPU", i, device.Name(), device.MaxComputeUnits(), "CUs")
		q, _ := context.CreateCommandQueue(device, 0)
		queues = append(queues, q)
	}
	if numDevices == 0 || *printVersion { return }
	log.SetFlags(log.Lmicroseconds)

	data, err := ioutil.ReadFile(Kernel)
	if err != nil { panic(err) }

	source := []string{string(data)}
	dagKernel, err := context.CreateKernelWithSource(source, "-legacy", "calc_dag_item")
	if err != nil { panic(err) }
	log.Println(dagKernel)

	conn, err := net.Dial("tcp", *pool); defer conn.Close()
	if err != nil {
		log.Println(err)
	}
	log.Println("Connected to", *pool)

	params := []string{*addr}
	login := jmsg{jhdr{1, "2.0"}, jbody{"eth_submitLogin", params}}
	data, jerr := json.Marshal(login)
	data = append(data, byte('\n'))
	conn.Write(data)
	log.Println("Waiting for new job...")

    var buf []byte
    reader := bufio.NewReader(conn)
    // skip json result:true message
    response := jhdr{99, ""}
    for ; response.Id != 0; {
        buf, _ = reader.ReadBytes('\n')
        jerr = json.Unmarshal(buf, &response)
        if jerr != nil { fmt.Println(jerr) }
	}
    var rcvd struct{Result []string `json:"result"`}
    jerr = json.Unmarshal(buf, &rcvd)
    seedHex := rcvd.Result[1]
    log.Println("Seed:", seedHex)

    seed, _ := hex.DecodeString(seedHex[2:]) 
    epoch := 0
	kf256 := sha3.ReHashK256()
    for ;!bytes.Equal(kf256.Data(), seed); epoch++ {
		kf256.Hash()
    }
	log.Println("Creating epoch", epoch, "cache")
    cache := makeCache(epoch, seed)
	log.Println("Created cache, size", len(cache)/MB, "MB")
    debug("%x\n",cache[len(cache)-8:])

    rand.Seed(time.Now().UnixNano())
/*
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
*/
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
