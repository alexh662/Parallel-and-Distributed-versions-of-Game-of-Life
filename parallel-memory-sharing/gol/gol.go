package gol

import "sync"

// Params provides the details of how to run the Game of Life and which image to load.
type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

type KeyPressesInterface struct {
	key rune
	mu  sync.Mutex
}

type EventInterface struct {
	event Event
	mu    sync.Mutex
}

func (k *KeyPressesInterface) Get() rune {
	k.mu.Lock()
	defer k.mu.Unlock()

	return k.key
}

func (k *KeyPressesInterface) Set(newKey rune) {
	k.mu.Lock()
	defer k.mu.Unlock()

	k.key = newKey
}

func (e *EventInterface) Get() Event {
	e.mu.Lock()
	defer e.mu.Unlock()

	return e.event
}

func (e *EventInterface) Set(newEvent Event) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.event = newEvent
}

func (e *EventInterface) Reset() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.event = nil
}

// Run starts the processing of Game of Life. It should initialise channels and goroutines.
func Run(p Params, events *EventInterface, keypresses *KeyPressesInterface) {

	//	TODO: Put the missing channels in here.

	var ioState IoInterface
	ioState.cond = sync.NewCond(&ioState.mu)
	go startIo(p, &ioState)

	distributorInterface := DistributorInterface{
		io:         &ioState,
		keyPresses: keypresses,
		events:     events,
	}

	distributor(p, &distributorInterface)
}
