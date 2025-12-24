package gol

import (
	"fmt"
	"sync"
	"time"

	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
	keyPresses <-chan rune
}

type boolInterface struct {
	boolean bool
	mu      sync.Mutex
}

type Worker struct {
	startY, endY, imageHeight, imageWidth int
	world                                 *[][]byte
	nextWorld                             *[][]byte
	wg                                    *sync.WaitGroup
}

func (w *Worker) run() {
	calculateNextStateOptimised(w.startY, w.endY, w.imageHeight, w.imageWidth, w.world, w.nextWorld)
	w.wg.Done()
}

func (k *boolInterface) setTrue() {
	k.mu.Lock()
	defer k.mu.Unlock()

	k.boolean = true
}

func (k *boolInterface) setFalse() {
	k.mu.Lock()
	defer k.mu.Unlock()

	k.boolean = false
}

func (k *boolInterface) get() bool {
	k.mu.Lock()
	defer k.mu.Unlock()

	return k.boolean
}

func calculateNextStateOptimised(startY, endY, imageHeight, imageWidth int, world, nextWorld *[][]byte) {
	size := endY - startY
	currentWorld := *world
	next := *nextWorld

	aliveCells := make([][]byte, size)
	for y := range aliveCells {
		aliveCells[y] = make([]byte, imageWidth)
	}

	top := startY - 1
	if top < 0 {
		top = imageHeight - 1
	} else if top > imageHeight-1 {
		top = 0
	}

	bottom := endY
	if bottom < 0 {
		bottom = imageHeight - 1
	} else if bottom > imageHeight-1 {
		bottom = 0
	}

	for x := 0; x < imageWidth; x++ {
		if currentWorld[top][x] == 255 {
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
		if currentWorld[bottom][x] == 255 {
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
			if currentWorld[y+startY][x] == 255 {
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

	for y := 0; y < size; y++ {
		for x := 0; x < imageWidth; x++ {
			sum := aliveCells[y][x]
			switch {
			case sum < 2:
				next[y+startY][x] = 0
			case sum == 2:
				next[y+startY][x] = currentWorld[y+startY][x]
			case sum == 3:
				next[y+startY][x] = 255
			default:
				next[y+startY][x] = 0
			}
		}
	}
}

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

func tickerLoop(getAliveCount *boolInterface, done chan struct{}) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			getAliveCount.setTrue()
		case <-done:
			return
		}
	}
}

func handleKeyPresses(c distributorChannels, quit *boolInterface, paused *boolInterface, savePgm *boolInterface, done <-chan struct{}) {
	for {
		select {
		case <-done:
			return
		case key := <-c.keyPresses:
			switch key {
			case 's':
				savePgm.setTrue()
			case 'q':
				quit.setTrue()
			case 'p':
				if paused.get() {
					paused.setFalse()
				} else {
					paused.setTrue()
				}
			}
		}
	}
}

func calculateCellsConcurrent(threads int, world, nextWorld *[][]byte, workers []*Worker, wg *sync.WaitGroup) {
	for i := 0; i < threads; i++ {
		workers[i].world = world
		workers[i].nextWorld = nextWorld
		wg.Add(1)
		go workers[i].run()
	}

	wg.Wait()
}

func outputFile(p Params, c distributorChannels, world [][]byte, turn int) {
	filename := fmt.Sprintf("%dx%dx%d", p.ImageWidth, p.ImageHeight, turn)
	c.ioCommand <- ioOutput
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

func pausedFunc(p Params, c distributorChannels, paused *boolInterface, quit *boolInterface, savePgm *boolInterface, world [][]byte, turn int) {
	c.events <- StateChange{turn, Paused}

	for paused.get() && !quit.get() {
		if savePgm.get() {
			outputFile(p, c, world, turn)
			savePgm.setFalse()
		}
	}

	if !quit.get() {
		c.events <- StateChange{turn, Executing}
	}

	return
}

func executeTurns(p Params, c distributorChannels, world, nextWorld *[][]byte, turn *int, quit *boolInterface, paused *boolInterface, savePgm *boolInterface, getAliveCount *boolInterface) {
	var workerWg sync.WaitGroup
	workers := make([]*Worker, p.Threads)

	for i := 0; i < p.Threads; i++ {
		startY := i * p.ImageHeight / p.Threads
		endY := (i + 1) * p.ImageHeight / p.Threads
		worker := Worker{
			startY:      startY,
			endY:        endY,
			imageHeight: p.ImageHeight,
			imageWidth:  p.ImageWidth,
			world:       world,
			nextWorld:   nextWorld,
			wg:          &workerWg,
		}
		workers[i] = &worker
	}

	for *turn < p.Turns {
		if paused.get() {
			pausedFunc(p, c, paused, quit, savePgm, *world, *turn)
		}

		if quit.get() {
			return
		}

		if getAliveCount.get() {
			c.events <- AliveCellsCount{CompletedTurns: *turn, CellsCount: len(calculateAliveCells(*world))}
			getAliveCount.setFalse()
		}

		if savePgm.get() {
			outputFile(p, c, *world, *turn)
			savePgm.setFalse()
		}

		calculateCellsConcurrent(p.Threads, world, nextWorld, workers, &workerWg)

		*turn++

		for y := 0; y < p.ImageHeight; y++ {
			for x := 0; x < p.ImageWidth; x++ {
				if (*world)[y][x] != (*nextWorld)[y][x] {
					c.events <- CellFlipped{*turn, util.Cell{X: x, Y: y}}
				}
			}
		}

		for y := 0; y < p.ImageHeight; y++ {
			copy((*world)[y], (*nextWorld)[y])
		}

		c.events <- TurnComplete{*turn}
	}
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {
	world := make([][]byte, p.ImageHeight)
	nextWorld := make([][]byte, p.ImageHeight)
	for y := 0; y < p.ImageHeight; y++ {
		world[y] = make([]byte, p.ImageWidth)
		nextWorld[y] = make([]byte, p.ImageWidth)
	}

	var quit, savePgm, getAliveCount, paused boolInterface
	turn := 0
	quit.setFalse()
	paused.setFalse()
	savePgm.setFalse()
	getAliveCount.setFalse()

	c.ioCommand <- ioInput
	c.ioFilename <- fmt.Sprintf("%dx%d", p.ImageWidth, p.ImageHeight)
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			world[y][x] = <-c.ioInput
		}
	}

	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			if world[y][x] == 255 {
				c.events <- CellFlipped{CompletedTurns: 0, Cell: util.Cell{X: x, Y: y}}
			}
		}
	}

	c.events <- StateChange{0, Executing}

	done := make(chan struct{})

	go tickerLoop(&getAliveCount, done)
	go handleKeyPresses(c, &quit, &paused, &savePgm, done)

	executeTurns(p, c, &world, &nextWorld, &turn, &quit, &paused, &savePgm, &getAliveCount)

	if p.Turns == 0 {
		nextWorld = world
	}

	close(done)

	finalCount := len(calculateAliveCells(nextWorld))
	c.events <- AliveCellsCount{CompletedTurns: turn, CellsCount: finalCount}

	c.events <- FinalTurnComplete{turn, calculateAliveCells(nextWorld)}

	outputFile(p, c, nextWorld, turn)

	c.events <- StateChange{turn, Quitting}

	close(c.events)
}
