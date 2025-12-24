package main

import (
	"flag"
	"log"
	"net"
	"net/rpc"

	"uk.ac.bris.cs/gameoflife/stubs"
)

var quit = make(chan struct{})

func calculateNextState(startY, endY, imageHeight, imageWidth int, world [][]byte) [][]byte {
	nextWorld := make([][]byte, endY-startY)

	for y := range nextWorld {
		nextWorld[y] = make([]byte, imageWidth)
	}

	for y := startY; y < endY; y++ {
		for x := 0; x < imageWidth; x++ {
			sum := 0
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if dx == 0 && dy == 0 {
						continue
					}
					ny := (y + dy + imageHeight) % imageHeight
					nx := (x + dx + imageWidth) % imageWidth
					if world[ny][nx] == 255 {
						sum++
					}
				}
			}

			switch {
			case sum < 2:
				nextWorld[y-startY][x] = 0
			case sum == 2:
				nextWorld[y-startY][x] = world[y][x]
			case sum == 3:
				nextWorld[y-startY][x] = 255
			default:
				nextWorld[y-startY][x] = 0
			}
		}
	}

	return nextWorld
}

// return a slice of each cell's total alive neighbours for a section of the world
func calculateNextStateOptimised(size, imageWidth int, section [][]byte) [][]byte {
	//var flippedCells []util.Cell

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

			//if world[y+startY][x] == 255 && (sum < 2 || sum > 3) {
			//	nextSection[y][x] = 0
			//	flippedCells = append(flippedCells, util.Cell{x, y + startY})
			//} else if world[y+startY][x] == 0 && (sum == 3) {
			//	nextSection[y][x] = 255
			//	flippedCells = append(flippedCells, util.Cell{x, y + startY})
			//} else {
			//	nextSection[y][x] = world[y+startY][x]
			//}
		}
	}

	return nextSection
	//return stubs.WorkerResType{nextSection, flippedCells}
}

// type struct interface for rpc calls communication between the worker and the broker
type CalculateNextStateOperation struct{}

// broker makes an rpc call to the worker, worker sending the next state of the world
func (s *CalculateNextStateOperation) Calculate(req stubs.WorkerRequest, res *stubs.WorkerResponse2) (err error) {
	res.Section = calculateNextStateOptimised(req.Size, req.ImageWidth, req.Section)
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
