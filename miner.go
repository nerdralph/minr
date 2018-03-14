package main

import (
	"log"
//	"time"
    "math/rand"

	"github.com/robvanmieghem/go-opencl/cl"
)

//HashRateReport is sent from the mining routines for giving combined information as output
type HashRateReport struct {
	MinerID  int
	HashRate float64
}

type MiningWork struct {
	Header []byte
}

type Solution struct {
    Nonce uint64
	Mix [32]byte
	Header [32]byte
}

// Miner actually mines :-)
type Miner struct {
	clDevice          *cl.Device
    work              *MiningWork
	minerID           int
    pool              *Client 
	GlobalItemSize    int
    run               bool
}

func (miner *Miner) mine() {
    // start with random nonce
    nonce := uint64(rand.Int63())

	context, err := cl.CreateContext([]*cl.Device{miner.clDevice})
	defer context.Release()

	commandQueue, err := context.CreateCommandQueue(miner.clDevice, 0)
	defer commandQueue.Release()

	program, err := context.CreateProgramWithSource([]string{kernelSource})
	if err != nil { log.Fatalln(miner.minerID, err) }
	defer program.Release()

	err = program.BuildProgram([]*cl.Device{miner.clDevice}, "")
	if err != nil { log.Fatalln(miner.minerID, err) }

	kernel, err := program.CreateKernel("search")
	if err != nil { log.Fatalln(miner.minerID, "CreateKernel error:", err) }
	defer kernel.Release()

	nonceOutObj, err := context.CreateEmptyBuffer(cl.MemReadWrite, 8)
	if err != nil { log.Fatalln(miner.minerID, "CreateBuffer error:", err) }
	defer nonceOutObj.Release()

	kernel.SetArgs(blockHeaderObj, nonceOutObj)

	localItemSize := 64
    globalItemSize := localItemSize * 8192

	log.Println("GPU", miner.minerID, "Initialized ")

	nonceOut := make([]byte, 8, 8)
	if _, err = commandQueue.EnqueueWriteBufferByte(nonceOutObj, true, 0, nonceOut, nil); err != nil {
		log.Fatalln(miner.minerID, "EnqueueWrite error", err)
	}
	for run {
        //start := time.Now()
        //work.Offset = nonce + uint64(miner.GlobalItemSize)

		//Copy input to kernel args
		if _, err = commandQueue.EnqueueWriteBufferByte(blockHeaderObj, true, 0, work.Header, nil); err != nil {
		    log.Fatalln(miner.minerID, "EnqueueWrite error", err)
		}

		//Run the kernel
		if _, err = commandQueue.EnqueueNDRangeKernel(kernel, nil, []int{globalItemSize}, []int{localItemSize}, nil); err != nil {
			log.Fatalln(miner.minerID, "-", err)
		}
		//Get output
		if _, err = commandQueue.EnqueueReadBufferByte(nonceOutObj, true, 0, nonceOut, nil); err != nil {
			log.Fatalln(miner.minerID, "-", err)
		}
		//Check if match found
		if nonceOut[0] {
			log.Println(miner.minerID, "Solution found!" )
			// Copy nonce to a new header.
			header := append([]byte(nil), work.Header[:80]...)
			for i := 0; i < 8; i++ {
				header[i+32] = nonceOut[i]
			}

			//Clear the output since it is dirty now
			nonceOut = make([]byte, 8, 8)
			if _, err = commandQueue.EnqueueWriteBufferByte(nonceOutObj, true, 0, nonceOut, nil); err != nil {
				log.Fatalln(miner.minerID, "-", err)
			}
		}
        // update hashrate
	}

}
