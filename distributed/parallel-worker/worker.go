package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/rpc"

	"uk.ac.bris.cs/gameoflife/stubs"
)

var quit = make(chan struct{})
var aboveWorker, belowWorker *rpc.Client
var aboveHalo = make(chan []byte, 1)
var belowHalo = make(chan []byte, 1)

func calculateNextStateOptimised1Thread(size, imageWidth int, section [][]byte) [][]byte {
	aliveCells := make([][]byte, size)
	for y := range aliveCells {
		aliveCells[y] = make([]byte, imageWidth)
	}

	for x := 0; x < imageWidth; x++ {
		if section[size-1][x] == 255 {
			for dx := -1; dx <= 1; dx++ {
				nx := x + dx
				if nx < 0 {
					nx = imageWidth - 1
				} else if nx > imageWidth-1 {
					nx = 0
				}
				aliveCells[0][nx]++
			}
		}
	}

	for x := 0; x < imageWidth; x++ {
		if section[0][x] == 255 {
			for dx := -1; dx <= 1; dx++ {
				nx := x + dx
				if nx < 0 {
					nx = imageWidth - 1
				} else if nx > imageWidth-1 {
					nx = 0
				}
				aliveCells[size-1][nx]++
			}
		}
	}

	for y := 0; y < size; y++ {
		for x := 0; x < imageWidth; x++ {
			if section[y][x] == 255 {
				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						if !(dx == 0 && dy == 0) {
							ny := y + dy

							if ny < 0 || ny > size-1 {
								continue
							}

							nx := x + dx

							if nx < 0 {
								nx = imageWidth - 1
							} else if nx > imageWidth-1 {
								nx = 0
							}

							aliveCells[ny][nx]++
						}
					}
				}
			}
		}
	}

	nextSection := make([][]byte, size)
	for y := range nextSection {
		nextSection[y] = make([]byte, imageWidth)
	}

	for y := 0; y < size; y++ {
		for x := 0; x < imageWidth; x++ {
			sum := aliveCells[y][x]

			switch {
			case sum < 2:
				nextSection[y][x] = 0
			case sum == 2:
				nextSection[y][x] = section[y][x]
			case sum == 3:
				nextSection[y][x] = 255
			default:
				nextSection[y][x] = 0
			}
		}
	}

	return nextSection
}

// return a slice of each cell's total alive neighbours for a section of the world
func calculateNextStateOptimised(size, imageWidth int, section [][]byte) [][]byte {
	aliveCells := make([][]byte, size)
	for y := range aliveCells {
		aliveCells[y] = make([]byte, imageWidth)
	}

	for x := 0; x < imageWidth; x++ {
		if section[0][x] == 255 {
			for dx := -1; dx <= 1; dx++ {
				nx := x + dx
				if nx < 0 {
					nx = imageWidth - 1
				} else if nx > imageWidth-1 {
					nx = 0
				}
				aliveCells[0][nx]++
			}
		}
	}

	for x := 0; x < imageWidth; x++ {
		if section[size+1][x] == 255 {
			for dx := -1; dx <= 1; dx++ {
				nx := x + dx
				if nx < 0 {
					nx = imageWidth - 1
				} else if nx > imageWidth-1 {
					nx = 0
				}
				aliveCells[size-1][nx]++
			}
		}
	}

	for y := 0; y < size; y++ {
		for x := 0; x < imageWidth; x++ {
			if section[y+1][x] == 255 {
				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						if !(dx == 0 && dy == 0) {
							ny := y + dy

							if ny < 0 || ny > size-1 {
								continue
							}

							nx := x + dx

							if nx < 0 {
								nx = imageWidth - 1
							} else if nx > imageWidth-1 {
								nx = 0
							}

							aliveCells[ny][nx]++
						}
					}
				}
			}
		}
	}

	nextSection := make([][]byte, size)
	for y := range nextSection {
		nextSection[y] = make([]byte, imageWidth)
	}

	for y := 0; y < size; y++ {
		for x := 0; x < imageWidth; x++ {
			sum := aliveCells[y][x]

			switch {
			case sum < 2:
				nextSection[y][x] = 0
			case sum == 2:
				nextSection[y][x] = section[y+1][x]
			case sum == 3:
				nextSection[y][x] = 255
			default:
				nextSection[y][x] = 0
			}
		}
	}

	return nextSection
}

func smallerWorker(size, imageWidth int, world [][]byte, out chan<- [][]byte) {
	out <- calculateNextStateOptimised(size, imageWidth, world)
}

func runSmallerWorkers1Worker(size, imageWidth, threads int, section [][]byte) [][]byte {
	nextSmallerSection := make([][]byte, size)
	for y := 0; y < size; y++ {
		nextSmallerSection[y] = make([]byte, imageWidth)
	}

	if threads > 1 {
		channels := make([]chan [][]byte, threads)
		for i := 0; i < threads; i++ {
			channels[i] = make(chan [][]byte, 1)
		}

		for i := 0; i < threads; i++ {
			smallerStartY := i * size / threads
			smallerEndY := (i + 1) * size / threads

			smallerSection := make([][]byte, smallerEndY-smallerStartY+2)

			if i == 0 {
				smallerSection[0] = section[size-1]
			} else {
				smallerSection[0] = section[smallerStartY-1]
			}

			if i == threads-1 {
				smallerSection[smallerEndY-smallerStartY+1] = section[0]
			} else {
				smallerSection[smallerEndY-smallerStartY+1] = section[smallerEndY]
			}

			for y := 1; y < smallerEndY-smallerStartY+1; y++ {
				smallerSection[y] = section[y+smallerStartY-1]
			}

			go smallerWorker(smallerEndY-smallerStartY, imageWidth, smallerSection, channels[i])
		}

		for i := 0; i < threads; i++ {
			workerSection := <-channels[i]
			smallerStartY := i * size / threads
			smallerEndY := (i + 1) * size / threads

			for y := 0; y < smallerEndY-smallerStartY; y++ {
				copy(nextSmallerSection[y+smallerStartY], workerSection[y])
			}
		}
	} else {
		nextSmallerSection = calculateNextStateOptimised1Thread(size, imageWidth, section)
	}

	return nextSmallerSection
}

func runSmallerWorkers(size, imageWidth, threads int, section [][]byte) [][]byte {
	nextSmallerSection := make([][]byte, size)
	for y := 0; y < size; y++ {
		nextSmallerSection[y] = make([]byte, imageWidth)
	}

	channels := make([]chan [][]byte, threads)
	for i := 0; i < threads; i++ {
		channels[i] = make(chan [][]byte, 1)
	}

	for i := 0; i < threads; i++ {
		smallerStartY := i * size / threads
		smallerEndY := (i + 1) * size / threads

		smallerSection := make([][]byte, smallerEndY-smallerStartY+2)

		for y := 0; y < smallerEndY-smallerStartY+2; y++ {
			smallerSection[y] = section[smallerStartY+y]
		}

		go smallerWorker(smallerEndY-smallerStartY, imageWidth, smallerSection, channels[i])
	}

	for i := 0; i < threads; i++ {
		workerSection := <-channels[i]
		smallerStartY := i * size / threads
		smallerEndY := (i + 1) * size / threads

		for y := 0; y < smallerEndY-smallerStartY; y++ {
			copy(nextSmallerSection[y+smallerStartY], workerSection[y])
		}
	}

	return nextSmallerSection
}

func sendHalo(region []byte, above bool) {
	request := stubs.HaloRequest{
		Region: region,
		Above:  above,
	}
	response := new(stubs.Empty)
	if above {
		if err_ := belowWorker.Call(stubs.ReceiveHaloRegionHandler, request, &response); err_ != nil {
			fmt.Println(err_)
		}
	} else {
		if err_ := aboveWorker.Call(stubs.ReceiveHaloRegionHandler, request, &response); err_ != nil {
			fmt.Println(err_)
		}
	}
}

// type struct interface for rpc calls communication between the worker and the broker
type CalculateNextStateOperation struct{}

func (s *CalculateNextStateOperation) Calculate(req stubs.ParallelWorkerRequest, res *stubs.WorkerResponse2) (err error) {
	res.Section = runSmallerWorkers1Worker(req.Size, req.ImageWidth, req.Threads, req.Section)
	return
}

// broker makes an rpc call to the worker, worker sending the next state of the world
func (s *CalculateNextStateOperation) CalculateHalo(req stubs.ParallelWorkerRequest, res *stubs.WorkerResponse2) (err error) {
	go sendHalo(req.Section[0], false)
	go sendHalo(req.Section[req.Size-1], true)

	extendedSection := make([][]byte, req.Size+2)
	nextSection := make([][]byte, req.Size)

	for i := 1; i < req.Size+1; i++ {
		extendedSection[i] = req.Section[i-1]
	}

	interiorSection := runSmallerWorkers(req.Size-2, req.ImageWidth, req.Threads, extendedSection[1:req.Size+1])
	for y := 0; y < req.Size-2; y++ {
		nextSection[y+1] = interiorSection[y]
	}

	extendedSection[0] = <-aboveHalo
	extendedSection[req.Size+1] = <-belowHalo

	nextSection[0] = calculateNextStateOptimised(1, req.ImageWidth, extendedSection[0:3])[0]
	nextSection[req.Size-1] = calculateNextStateOptimised(1, req.ImageWidth, extendedSection[req.Size-1:req.Size+2])[0]

	//extendedSection[0] = <-aboveHalo
	//extendedSection[req.Size+1] = <-belowHalo
	//
	//for y := 1; y < req.Size+1; y++ {
	//	extendedSection[y] = req.Section[y-1]
	//}

	//res.Section = runSmallerWorkers(req.Size, req.ImageWidth, req.Threads, extendedSection)
	res.Section = nextSection
	return
}

func (s *CalculateNextStateOperation) SetupHalo(req stubs.HaloSetupRequest, res *stubs.Empty) (err error) {
	aboveWorker, err = rpc.Dial("tcp", fmt.Sprintf("127.0.0.1%v", req.AbovePort))
	belowWorker, err = rpc.Dial("tcp", fmt.Sprintf("127.0.0.1%v", req.BelowPort))
	if err != nil {
		fmt.Println(err)
	}
	return
}

func (s *CalculateNextStateOperation) ReceiveHaloRegion(req stubs.HaloRequest, res *stubs.Empty) (err error) {
	if req.Above {
		aboveHalo <- req.Region
	} else {
		belowHalo <- req.Region
	}
	return
}

// broker makes an rpc call to worker to quit the worker
func (s *CalculateNextStateOperation) Quit(_ stubs.Empty, _ *stubs.Empty) (err error) {
	quit <- struct{}{}
	return
}

func main() {
	pAddr := flag.String("port", ":8040", "Port to listen to")
	flag.Parse()
	//creates an rpc server
	rpc.Register(&CalculateNextStateOperation{})
	//puts server on address
	listener, err := net.Listen("tcp", *pAddr)
	if err != nil {
		log.Fatal("net.Listen:", err)
	}
	//when main finishes listener will quit
	defer listener.Close()

	//rpc will listen on this port
	go rpc.Accept(listener)

	//waits for quit signal
	<-quit
}
