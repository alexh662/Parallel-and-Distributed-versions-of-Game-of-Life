package stubs

import (
	"uk.ac.bris.cs/gameoflife/util"
)

var (
	ExecuteHandler        = "ExecuteTurnsOperation.Execute"
	CalculateAliveHandler = "ExecuteTurnsOperation.CalculateAlive"
	KeyPressHandler       = "ExecuteTurnsOperation.HandleKeyPress"
)

var (
	CalculateNextStateHaloHandler = "CalculateNextStateOperation.CalculateHalo"
	CalculateNextStateHandler     = "CalculateNextStateOperation.Calculate"
	SetupHaloHandler              = "CalculateNextStateOperation.SetupHalo"
	ReceiveHaloRegionHandler      = "CalculateNextStateOperation.ReceiveHaloRegion"
	QuitHandler                   = "CalculateNextStateOperation.Quit"
)

var (
	SDLHandler = "SDLOperation.SDL"
)

type BrokerResponse struct {
	World          [][]byte
	Cells          []util.Cell
	CompletedTurns int
	Paused         bool
}

type BrokerRequest struct {
	World       [][]byte
	CurrentTurn int
	ImageHeight int
	ImageWidth  int
	Turns       int
	Key         rune
	Workers     int
	Threads     int
}

type WorkerResType struct {
	Section [][]byte
	Cells   []util.Cell
}

type WorkerResponse struct {
	Res WorkerResType
}

type WorkerResponse2 struct {
	Section [][]byte
}

type WorkerRequest struct {
	Section    [][]byte
	ImageWidth int
	Size       int
}

type ParallelWorkerRequest struct {
	Section    [][]byte
	ImageWidth int
	Size       int
	Threads    int
}

type HaloSetupRequest struct {
	AbovePort string
	BelowPort string
}

type HaloRequest struct {
	Region []byte
	Above  bool
}

type ClientRequest struct {
	FlippedCells   []util.Cell
	CompletedTurns int
}

type Empty struct{}
