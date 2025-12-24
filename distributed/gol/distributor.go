package gol

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
	"sync"
	"time"

	"uk.ac.bris.cs/gameoflife/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

// interface of channels for communication with io
type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
	keyPresses <-chan rune
}

// interface for communication with sdl
type SDLOperation struct{}

type SDLInfo struct {
	cells []util.Cell
	turns int
}

var sdlChan chan SDLInfo

func updateSDL(c distributorChannels, done <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case info := <-sdlChan:
			c.events <- CellsFlipped{info.turns, info.cells}
			c.events <- TurnComplete{info.turns}
		case <-done:
			return
		}
	}

}

func (s *SDLOperation) SDL(req stubs.ClientRequest, res *stubs.Empty) (err error) {
	sdlChan <- SDLInfo{
		cells: req.FlippedCells,
		turns: req.CompletedTurns,
	}

	return
}

func handleKeyPresses(c distributorChannels, p Params, client *rpc.Client, done <-chan struct{}, doneWG *sync.WaitGroup) {
	defer doneWG.Done()

	for {
		select {
		case <-done:
			return
		//io listens on keypresses and sends them to distributor using the events channel
		case key := <-c.keyPresses:
			switch key {
			//if a key is pressed, the client makes an rpc call to broker containing the key pressed
			//broker does keypress handling then sends a response back to client containing information of the new state
			case 's':
				request := stubs.BrokerRequest{World: nil, CurrentTurn: 0, ImageHeight: 0, ImageWidth: 0, Turns: 0, Key: key}
				response := new(stubs.BrokerResponse)
				client.Call(stubs.KeyPressHandler, request, response)
				world := response.World
				turns := response.CompletedTurns
				// 's' logic is handled on the client side
				outputFile(p, c, world, turns)
			case 'q':
				request := stubs.BrokerRequest{World: nil, CurrentTurn: 0, ImageHeight: 0, ImageWidth: 0, Turns: 0, Key: key}
				response := new(stubs.BrokerResponse)
				client.Call(stubs.KeyPressHandler, request, response)
			case 'k':
				request := stubs.BrokerRequest{World: nil, CurrentTurn: 0, ImageHeight: 0, ImageWidth: 0, Turns: 0, Key: key}
				response := new(stubs.BrokerResponse)
				client.Call(stubs.KeyPressHandler, request, response)
			case 'p':
				request := stubs.BrokerRequest{World: nil, CurrentTurn: 0, ImageHeight: 0, ImageWidth: 0, Turns: 0, Key: key}
				response := new(stubs.BrokerResponse)
				client.Call(stubs.KeyPressHandler, request, response)
				//changing state for pause status
				if response.Paused {
					c.events <- StateChange{response.CompletedTurns, Paused}
				} else {
					c.events <- StateChange{response.CompletedTurns, Executing}
				}
			}
		}
	}
}

// ticker function to output the state of the world every 2 seconds
func ticker(c distributorChannels, p Params, client *rpc.Client, done chan struct{}, doneWG *sync.WaitGroup) {
	defer doneWG.Done()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			//rpc call from client to broker requesting the number of alive cells to output
			request := stubs.BrokerRequest{World: nil, CurrentTurn: 0, ImageHeight: p.ImageHeight, ImageWidth: p.ImageWidth, Turns: p.Turns}
			response := new(stubs.BrokerResponse)
			client.Call(stubs.CalculateAliveHandler, request, response)
			c.events <- AliveCellsCount{CompletedTurns: response.CompletedTurns, CellsCount: len(response.Cells)}
		case <-done:
			return
		}
	}
}

// function to create an output file for pgm image requests
func outputFile(p Params, c distributorChannels, world [][]byte, turn int) {
	c.ioCommand <- ioOutput
	filename := fmt.Sprintf("%dx%dx%d", p.ImageWidth, p.ImageHeight, turn)
	c.ioFilename <- filename
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[y][x]
		}
	}

	c.ioCommand <- ioCheckIdle
	<-c.ioIdle
	c.events <- ImageOutputComplete{turn, filename}
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {
	brokerAddr := "127.0.0.1:8030"
	//brokerAddr := "18.234.218.164:8030"
	client, err := rpc.Dial("tcp", brokerAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	// TODO: Create a 2D slice to store the world.
	world := make([][]byte, p.ImageHeight)
	for i := range world {
		world[i] = make([]byte, p.ImageWidth)
	}

	c.ioCommand <- ioInput
	c.ioFilename <- fmt.Sprintf("%dx%d", p.ImageWidth, p.ImageHeight)
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			world[y][x] = <-c.ioInput
		}
	}

	turn := 0

	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			if world[y][x] == 255 {
				c.events <- CellFlipped{0, util.Cell{X: x, Y: y}}
			}
		}
	}

	c.events <- StateChange{turn, Executing}

	done := make(chan struct{})
	sdlChan = make(chan SDLInfo, 500000)
	var doneWG sync.WaitGroup

	controllerAddr := "127.0.0.1:8020"
	//controllerAddr := "192.168.0.21:8020"
	rpc.Register(&SDLOperation{})
	listener, err := net.Listen("tcp", controllerAddr)
	if err == nil {
		defer listener.Close()
		go rpc.Accept(listener)
	}

	doneWG.Add(3)
	go ticker(c, p, client, done, &doneWG)
	go handleKeyPresses(c, p, client, done, &doneWG)
	go updateSDL(c, done, &doneWG)

	if p.Workers < 1 {
		p.Workers = 1
	}

	request := stubs.BrokerRequest{
		World:       world,
		CurrentTurn: turn,
		ImageHeight: p.ImageHeight,
		ImageWidth:  p.ImageWidth,
		Turns:       p.Turns,
		Workers:     p.Workers,
		Threads:     p.Threads,
	}
	response := new(stubs.BrokerResponse)

	// execute all turns in game of life
	client.Call(stubs.ExecuteHandler, request, response)
	world = response.World
	finalTurn := response.CompletedTurns

	close(done)
	doneWG.Wait()

	outputFile(p, c, world, finalTurn)

	// TODO: Report the final state using FinalTurnCompleteEvent.
	c.events <- FinalTurnComplete{finalTurn, response.Cells}

	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{finalTurn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
