package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"sync"
	"time"

	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

var quit boolInterface
var quitChan = make(chan struct{})
var quitWg, collectInfo, handledPause sync.WaitGroup
var turnChannel = make(chan int)
var client *rpc.Client

// creating type structs for world, turn and boolean interface and giving them their own mutex
type World struct {
	world [][]byte
	mu    sync.Mutex
}

type Turn struct {
	turn int
	mu   sync.Mutex
}

type boolInterface struct {
	boolean bool
	mu      sync.Mutex
}

// functions to use mutexes for world, turns, get world, get turn, set world, set turn, increment turn,
// set boolean interface true/false and get boolean interface
func (w *World) set(newWorld [][]byte) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.world = cloneWorld(newWorld)
}

func (w *World) get() [][]byte {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.world
}

func (t *Turn) set(num int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.turn = num
}

func (t *Turn) increment() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.turn++
}

func (t *Turn) get() int {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.turn
}

func (b *boolInterface) setTrue() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.boolean = true
}

func (b *boolInterface) setFalse() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.boolean = false
}

func (b *boolInterface) get() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.boolean
}

// returns a list of the alive cells
func calculateAliveCells(world [][]byte) []util.Cell {
	var cells []util.Cell

	for y, r := range world {
		for x, c := range r {
			if c == 255 {
				cells = append(cells, util.Cell{X: x, Y: y})
			}
		}
	}

	return cells
}

// clones the world
func cloneWorld(w [][]byte) [][]byte {
	out := make([][]byte, len(w))
	for i := range w {
		out[i] = append([]byte(nil), w[i]...)
	}
	return out
}

// sends an rpc call to close the worker ports
func closeWorkers(workers []*rpc.Client) {
	req := stubs.Empty{}
	res := stubs.Empty{}

	for _, worker := range workers {
		if err := worker.Call(stubs.QuitHandler, req, &res); err != nil {
			fmt.Println(err)
		}
		worker.Close()
	}
}

// connects to all workers
func setupWorkers(workerNum int) []*rpc.Client {
	serverAddr := "127.0.0.1"
	workerPorts := [8]string{":8040", ":8050", ":8060", ":8070", ":8080", ":8090", ":9030", ":9040"}

	workers := make([]*rpc.Client, workerNum)

	for i := 0; i < workerNum; i++ {
		worker, err := rpc.Dial("tcp", fmt.Sprintf("%v%v", serverAddr, workerPorts[i]))
		if err != nil {
			fmt.Println(err)
		}

		//fmt.Printf("connected to worker %v\n", workerPorts[i])

		if workerNum > 1 {
			request := stubs.HaloSetupRequest{
				AbovePort: workerPorts[(i-1+workerNum)%workerNum],
				BelowPort: workerPorts[(i+1)%workerNum],
			}
			response := new(stubs.Empty)
			if err = worker.Call(stubs.SetupHaloHandler, request, &response); err != nil {
				fmt.Println(err)
			}
		}

		workers[i] = worker
	}

	return workers
}

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

// sends an rpc call to the workers with information about the state, so workers calculate the next state for a section
func runWorker(imageWidth, threads int, halo bool, worker *rpc.Client, section [][]byte, out chan<- [][]byte) {
	//req := stubs.WorkerRequest{
	//	Section:    section,
	//	ImageWidth: imageWidth,
	//	Size:       size,
	//}
	req := stubs.ParallelWorkerRequest{
		Section:    section,
		ImageWidth: imageWidth,
		Size:       len(section),
		Threads:    threads,
	}
	res := new(stubs.WorkerResponse2)

	handler := stubs.CalculateNextStateHaloHandler
	if !halo {
		handler = stubs.CalculateNextStateHandler
	}

	//if err := worker.Call(handler, req, res); err != nil {
	//	fmt.Println(err)
	//}

	errChan := make(chan error, 1)

	go func() {
		errChan <- worker.Call(handler, req, res)
	}()

	select {
	case err := <-errChan:
		if err != nil {
			fmt.Println("worker error, calculate locally: ", err)
			slice := calculateNextStateOptimised(len(section), imageWidth, section)
			out <- slice
			return
		}
		out <- res.Section
		return
	case <-time.After(1 * time.Second):
		fmt.Println("timeout, calculate locally")
		slice := calculateNextStateOptimised(len(section), imageWidth, section)
		out <- slice
		return
	}

	//out <- res.Res
	//out <- res.Section
}

// creates worker channels, receives the calculated next state for each slice from each worker
func calculateNextWorld(imageHeight, imageWidth, turn, threads int, world [][]byte, workers []*rpc.Client, workerNum int) [][]byte {
	nextWorld := make([][]byte, 0, imageHeight)
	//var cellsFlipped []util.Cell

	workerChannels := make([]chan [][]byte, workerNum)
	for i := 0; i < workerNum; i++ {
		workerChannels[i] = make(chan [][]byte)
	}

	if workerNum == 1 {
		go runWorker(imageWidth, threads, false, workers[0], world, workerChannels[0])
	} else {
		for i := 0; i < workerNum; i++ {
			startY := i * imageHeight / workerNum
			endY := (i + 1) * imageHeight / workerNum

			go runWorker(imageWidth, threads, true, workers[i], world[startY:endY], workerChannels[i])
		}
	}

	for i := 0; i < workerNum; i++ {
		res := <-workerChannels[i]
		nextWorld = append(nextWorld, res...)
		//cellsFlipped = append(cellsFlipped, res.Cells...)
	}

	//request := stubs.ClientRequest{cellsFlipped, turn}
	//response := new(stubs.Empty)
	//client.Call(stubs.SDLHandler, request, &response)

	return nextWorld
}

// type struct for rpc calls between broker and worker
type ExecuteTurnsOperation struct {
	world      World
	turn       Turn
	quitClient boolInterface
	pause      boolInterface
}

// execute loop
func (s *ExecuteTurnsOperation) Execute(req stubs.BrokerRequest, res *stubs.BrokerResponse) (err error) {
	//wait group for quit is waiting for a quit signal
	if req.Turns == 0 {
		res.World = req.World
		res.CompletedTurns = req.CurrentTurn
		res.Cells = calculateAliveCells(req.World)
		return
	}

	quitWg.Add(1)
	defer quitWg.Done()
	localControllerAddr := "127.0.0.1:8020"
	//localControllerAddr := "92.234.18.142:8020"
	localController, err := rpc.Dial("tcp", localControllerAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer localController.Close()
	client = localController

	workers := setupWorkers(req.Workers)

	s.world.set(cloneWorld(req.World))
	s.turn.set(req.CurrentTurn)
	s.quitClient.setFalse()

	currentTurn := req.CurrentTurn
	for currentTurn < req.Turns && !s.quitClient.get() {
		currentWorld := cloneWorld(s.world.get())
		nextWorld := calculateNextWorld(req.ImageHeight, req.ImageWidth, currentTurn+1, req.Threads, currentWorld, workers, req.Workers)

		s.world.set(nextWorld)
		s.turn.increment()

		currentTurn++

		var cellsFlipped []util.Cell
		for y := 0; y < req.ImageHeight; y++ {
			for x := 0; x < req.ImageWidth; x++ {
				if nextWorld[y][x] != currentWorld[y][x] {
					cellsFlipped = append(cellsFlipped, util.Cell{X: x, Y: y})
				}
			}
		}

		request := stubs.ClientRequest{FlippedCells: cellsFlipped, CompletedTurns: currentTurn}
		response := new(stubs.Empty)
		if err = localController.Call(stubs.SDLHandler, request, &response); err != nil {
			fmt.Println(err)
		}

		//pause logic
		if s.pause.get() {
			collectInfo.Done()
			for s.pause.get() && !s.quitClient.get() {
				continue
			}
			if s.quitClient.get() {
				s.pause.setFalse()
			}
		}
	}

	res.World = cloneWorld(s.world.get())
	res.CompletedTurns = s.turn.get()
	res.Cells = calculateAliveCells(s.world.get())
	s.quitClient.setFalse()

	if quit.get() {
		closeWorkers(workers)
		quitChan <- struct{}{}
	}

	return
}

// request the current world from client, sends back the alive cells and the turns
func (s *ExecuteTurnsOperation) CalculateAlive(req stubs.BrokerRequest, res *stubs.BrokerResponse) (err error) {
	if req.World != nil {
		res.Cells = calculateAliveCells(req.World)
		res.CompletedTurns = req.CurrentTurn
		return
	}

	world := s.world.get()
	turns := s.turn.get()

	res.CompletedTurns = turns
	res.Cells = calculateAliveCells(world)
	return
}

// gets keypresses throught the client rpc call
func (s *ExecuteTurnsOperation) HandleKeyPress(req stubs.BrokerRequest, res *stubs.BrokerResponse) (err error) {
	switch req.Key {
	case 'p':
		if s.pause.get() {
			handledPause.Wait()
			handledPause.Add(1)
			res.CompletedTurns = s.turn.get()
			s.pause.setFalse()
			handledPause.Done()
		} else {
			handledPause.Wait()
			handledPause.Add(1)
			collectInfo.Add(1)
			s.pause.setTrue()
			collectInfo.Wait()
			res.CompletedTurns = s.turn.get()
			handledPause.Done()
		}

		res.Paused = s.pause.get()
	//for 's', sends back the world and the completed turns
	case 's':
		res.World = s.world.get()
		res.CompletedTurns = s.turn.get()
	//quit logic for 'q' ans for 'k', pause logic for 'p'
	case 'q':
		s.pause.setFalse()
		s.quitClient.setTrue()
	case 'k':
		s.quitClient.setTrue()
		quit.setTrue()
	}
	return
}

func main() {
	quit.setFalse()
	pAddr := flag.String("port", ":8030", "Port to listen to")
	flag.Parse()

	rpc.Register(&ExecuteTurnsOperation{})
	listener, _ := net.Listen("tcp", *pAddr)
	defer listener.Close()
	go rpc.Accept(listener)

	<-quitChan
	quitWg.Wait()
	time.Sleep(2 * time.Second)
	listener.Close()
}
