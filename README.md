# minr
GPU miner for ethereum, optimized for AMD GPUs. 

### Prerequisites
* gol version 1.6.0 or above, check with `go version`
* opencl libraries on the library path

```
apt-get install -y git ocl-icd-libopencl1 opencl-headers golang
go get github.com/nerdralph/minr
```

## Run
```
gominer
```

Usage:
```
gominer -h
```

## FAQ

- *ERROR fetching work - Get http://localhost:9980/miner/headerforwork: dial tcp 127.0.0.1:9980: connection refused*

  Make sure `siad` is running

- What is `siad`?

  Check the sia documentation


## Support development

Donations can be made to:

ETH: 

