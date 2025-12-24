package gol

import (
	"fmt"
	"sync"
	"time"

	"uk.ac.bris.cs/gameoflife/util"
)

type DistributorInterface struct {
	io         *IoInterface
	keyPresses *KeyPressesInterface
	events     *EventInterface
}

type Turn struct {
	turn int
	mu   sync.RWMutex
}

type boolInterface struct {
	boolean bool
	mu      sync.Mutex
}

type stringInterface struct {
	string string
	mu     sync.RWMutex
}

func (s *stringInterface) set(x string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.string = x
}

func (s *stringInterface) get() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.string
}

type Section struct {
	startY int
	data   [][]byte
}

type Task struct {
	startY, endY int
	world        [][]byte
	nextWorld    *[][]byte
	wg           *sync.WaitGroup
}

func (t *Turn) init() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.turn = 0
}

func (t *Turn) get() int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.turn
}

func (t *Turn) increment() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.turn++
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

func copyWorld(w [][]byte) [][]byte {
	n := make([][]byte, len(w))
	for y := range w {
		n[y] = make([]byte, len(w[y]))
		copy(n[y], w[y])
	}
	return n
}

func setEvent(events *EventInterface, e Event) {
	for events.Get() != nil {
	}
	events.Set(e)
}

func calculateNextStateOptimised(startY, endY, imageHeight, imageWidth int, world [][]byte) [][]byte {
	aliveCells := make([][]byte, endY-startY)
	for y := range aliveCells {
		aliveCells[y] = make([]byte, imageWidth)
	}

	top := startY - 1
	if top < 0 {
		top = imageHeight - 1
	}

	bottom := endY
	if bottom > imageHeight-1 {
		bottom = 0
	}

	//top := (startY - 1 + imageHeight) % imageHeight
	//bottom := endY % imageHeight

	for x := 0; x < imageWidth; x++ {
		if world[top][x] == 255 {
			for dx := -1; dx <= 1; dx++ {
				//nx := (x + dx + imageWidth) % imageWidth
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
		if world[bottom][x] == 255 {
			for dx := -1; dx <= 1; dx++ {
				//nx := (x + dx + imageWidth) % imageWidth
				nx := x + dx
				if nx < 0 {
					nx = imageWidth - 1
				} else if nx > imageWidth-1 {
					nx = 0
				}
				aliveCells[endY-startY-1][nx]++
			}
		}
	}

	for y := 0; y < endY-startY; y++ {
		for x := 0; x < imageWidth; x++ {
			if world[y+startY][x] == 255 {
				for dy := -1; dy <= 1; dy++ {
					for dx := -1; dx <= 1; dx++ {
						if !(dx == 0 && dy == 0) {
							ny := y + dy

							if ny < 0 || ny > endY-startY-1 {
								continue
							}

							nx := x + dx
							//nx := (x + dx + imageWidth) % imageWidth

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

	nextSection := make([][]byte, endY-startY)
	for y := range nextSection {
		nextSection[y] = make([]byte, imageWidth)
	}

	for y := 0; y < endY-startY; y++ {
		for x := 0; x < imageWidth; x++ {
			sum := aliveCells[y][x]
			switch {
			case sum < 2:
				nextSection[y][x] = 0
			case sum == 2:
				nextSection[y][x] = world[y+startY][x]
			case sum == 3:
				nextSection[y][x] = 255
			default:
				nextSection[y][x] = 0
			}
		}
	}

	return nextSection
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

func tickerLoop(getAliveCount *boolInterface, done *boolInterface) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			getAliveCount.setTrue()
		}
		if done.get() {
			return
		}
	}
}

func handleKeyPresses(d *DistributorInterface, quit *boolInterface, paused *boolInterface, savePgm *boolInterface, done *boolInterface) {
	for {
		if done.get() {
			return
		}
		switch d.keyPresses.Get() {
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
		d.keyPresses.Set('a')
	}

}

func setIoCommand(ioInterface *IoInterface, command ioCommand) {
	ioInterface.mu.Lock()
	ioInterface.command = command
	ioInterface.request = true
	ioInterface.idle.setFalse()
	ioInterface.cond.Signal()
	ioInterface.mu.Unlock()
}

func worker(startY, endY, imageHeight, imageWidth int, world [][]byte, nextWorld *[][]byte, wg *sync.WaitGroup) {
	section := calculateNextStateOptimised(startY, endY, imageHeight, imageWidth, world)
	for y := startY; y < endY; y++ {
		copy((*nextWorld)[y], section[y-startY])
	}
	wg.Done()
}

func calculateCellsConcurrent(world [][]byte, threads, imageHeight, imageWidth int) [][]byte {
	nextWorld := make([][]byte, imageHeight)
	for y := range nextWorld {
		nextWorld[y] = make([]byte, imageWidth)
	}

	var wg sync.WaitGroup

	for i := 0; i < threads; i++ {
		startY := i * imageHeight / threads
		endY := (i + 1) * imageHeight / threads
		wg.Add(1)
		go worker(startY, endY, imageHeight, imageWidth, world, &nextWorld, &wg)
	}

	wg.Wait()

	return nextWorld
}

func outputFile(p Params, d *DistributorInterface, world [][]byte, turn int) {
	filename := fmt.Sprintf("%dx%dx%d", p.ImageWidth, p.ImageHeight, turn)
	d.io.filename.set(filename)

	buf := make([]byte, p.ImageHeight*p.ImageWidth)
	idx := 0
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			buf[idx] = world[y][x]
			idx++
		}
	}

	d.io.mu.Lock()
	d.io.buffer = buf
	d.io.mu.Unlock()

	setIoCommand(d.io, ioOutput)
	for !d.io.idle.get() {

	}

	setEvent(d.events, ImageOutputComplete{turn, filename})
}

func pausedFunc(p Params, d *DistributorInterface, paused *boolInterface, quit *boolInterface, savePgm *boolInterface, world [][]byte, turn int) {
	setEvent(d.events, StateChange{turn, Paused})

	for paused.get() && !quit.get() {
		if savePgm.get() {
			outputFile(p, d, world, turn)
			savePgm.setFalse()
		}
	}

	if !quit.get() {
		setEvent(d.events, StateChange{turn, Executing})
	}

	return
}

func executeTurns(p Params, d *DistributorInterface, world, currentWorld [][]byte, turn *Turn, quit *boolInterface, paused *boolInterface, savePgm *boolInterface, getAliveCount *boolInterface) [][]byte {
	t := turn.get()

	for t < p.Turns {
		if paused.get() {
			pausedFunc(p, d, paused, quit, savePgm, currentWorld, t)
		}

		if quit.get() {
			return world
		}

		if getAliveCount.get() {
			setEvent(d.events, AliveCellsCount{CompletedTurns: t, CellsCount: len(calculateAliveCells(world))})
			getAliveCount.setFalse()
		}

		if savePgm.get() {
			outputFile(p, d, world, t)
			savePgm.setFalse()
		}

		world = calculateCellsConcurrent(world, p.Threads, p.ImageHeight, p.ImageWidth)

		turn.increment()
		t++

		var cells []util.Cell
		for y := 0; y < p.ImageHeight; y++ {
			for x := 0; x < p.ImageWidth; x++ {
				if world[y][x] != currentWorld[y][x] {
					cells = append(cells, util.Cell{X: x, Y: y})
				}
			}
		}

		setEvent(d.events, CellsFlipped{t, cells})

		currentWorld = world

		setEvent(d.events, TurnComplete{t})
	}

	return world
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, d *DistributorInterface) {
	world := make([][]byte, p.ImageHeight)
	for y := 0; y < p.ImageHeight; y++ {
		world[y] = make([]byte, p.ImageWidth)
	}

	var turn Turn
	var quit, savePgm, getAliveCount, paused boolInterface
	turn.init()
	quit.setFalse()
	paused.setFalse()
	savePgm.setFalse()
	getAliveCount.setFalse()
	currentWorld := world

	d.io.filename.set(fmt.Sprintf("%dx%d", p.ImageWidth, p.ImageHeight))
	setIoCommand(d.io, ioInput)
	for !d.io.idle.get() {
	}

	d.io.mu.Lock()
	buf := make([]byte, p.ImageHeight*p.ImageWidth)
	copy(buf, d.io.buffer)
	d.io.mu.Unlock()
	idx := 0
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			currentWorld[y][x] = buf[idx]
			idx++
		}
	}

	var cells []util.Cell
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			if currentWorld[y][x] == 255 {
				cells = append(cells, util.Cell{X: x, Y: y})
			}
		}
	}

	setEvent(d.events, CellsFlipped{0, cells})

	setEvent(d.events, StateChange{0, Executing})

	var done boolInterface
	done.setFalse()

	go tickerLoop(&getAliveCount, &done)
	if d.keyPresses != nil {
		go handleKeyPresses(d, &quit, &paused, &savePgm, &done)
	}

	world = executeTurns(p, d, world, currentWorld, &turn, &quit, &paused, &savePgm, &getAliveCount)

	done.setTrue()

	finalCount := len(calculateAliveCells(world))
	finalTurn := turn.get()
	setEvent(d.events, AliveCellsCount{CompletedTurns: finalTurn, CellsCount: finalCount})

	setEvent(d.events, FinalTurnComplete{finalTurn, calculateAliveCells(world)})

	outputFile(p, d, world, finalTurn)

	setEvent(d.events, StateChange{finalTurn, Quitting})
}
