package sdl

import (
	"log"
	"time"

	"github.com/veandco/go-sdl2/sdl"
	"uk.ac.bris.cs/gameoflife/gol"
	"uk.ac.bris.cs/gameoflife/util"
)

const FPS = 60

func Run(p gol.Params, events *gol.EventInterface, keyPresses *gol.KeyPressesInterface) {
	w := NewWindow(int32(p.ImageWidth), int32(p.ImageHeight))
	defer w.Destroy()
	dirty := false
	refreshTicker := time.NewTicker(time.Second / time.Duration(FPS))
	avgTurns := util.NewAvgTurns()

sdl:
	for {
		event := events.Get()
		if event != nil {
			switch e := event.(type) {
			case gol.CellFlipped:
				w.FlipPixel(e.Cell.X, e.Cell.Y)
			case gol.CellsFlipped:
				for _, cell := range e.Cells {
					w.FlipPixel(cell.X, cell.Y)
				}
			case gol.TurnComplete:
				dirty = true
			case gol.AliveCellsCount:
				log.Printf(
					"[Event] Completed Turns %-8v %-20v Avg%+5v turns/sec\n",
					event.GetCompletedTurns(),
					event,
					avgTurns.TurnsPerSec(event.GetCompletedTurns()),
				)
			case gol.FinalTurnComplete:
				log.Printf("[Event] Completed Turns %-8v %v\n", event.GetCompletedTurns(), event)
			case gol.ImageOutputComplete:
				log.Printf("[Event] Completed Turns %-8v %v\n", event.GetCompletedTurns(), event)
			case gol.StateChange:
				log.Printf("[Event] Completed Turns %-8v %v\n", event.GetCompletedTurns(), event)
				if e.NewState == gol.Quitting {
					break sdl
				}
			}
			events.Reset()
		}
		select {
		case <-refreshTicker.C:
			event := w.PollEvent()
			if event != nil {
				switch e := event.(type) {
				case *sdl.QuitEvent:
					keyPresses.Set('q')
				case *sdl.KeyboardEvent:
					switch e.Keysym.Sym {
					case sdl.K_ESCAPE:
						keyPresses.Set('q')
					case sdl.K_p:
						keyPresses.Set('p')
					case sdl.K_s:
						keyPresses.Set('s')
					case sdl.K_q:
						keyPresses.Set('q')
					case sdl.K_k:
						keyPresses.Set('k')
					}
				}
			}
			if dirty {
				w.RenderFrame()
				dirty = false
			}
		default:
			//time.Sleep(time.Nanosecond)
		}
	}
}

func RunHeadless(events *gol.EventInterface) {
	avgTurns := util.NewAvgTurns()
	for {
		event := events.Get()
		if event != nil {
			switch e := event.(type) {
			case gol.AliveCellsCount:
				log.Printf(
					"[Event] Completed Turns %-8v %-20v Avg%+5v turns/sec\n",
					event.GetCompletedTurns(),
					event,
					avgTurns.TurnsPerSec(event.GetCompletedTurns()),
				)
			case gol.FinalTurnComplete:
				log.Printf("[Event] Completed Turns %-8v %v\n", event.GetCompletedTurns(), "Final Turn Complete")
			case gol.ImageOutputComplete:
				log.Printf("[Event] Completed Turns %-8v %v\n", event.GetCompletedTurns(), event)
			case gol.StateChange:
				log.Printf("[Event] Completed Turns %-8v %v\n", event.GetCompletedTurns(), event)
				if e.NewState == gol.Quitting {
					break
				}
			}
		}
		//time.Sleep(time.Nanosecond)
	}
}
