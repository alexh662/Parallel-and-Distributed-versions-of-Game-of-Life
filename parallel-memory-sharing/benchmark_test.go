package main

import (
	"fmt"
	"os"
	"testing"
	"time"

	"uk.ac.bris.cs/gameoflife/gol"
)

const benchLength = 1000

func BenchmarkGol(b *testing.B) {
	for threads := 1; threads <= 16; threads++ {
		p := gol.Params{
			Turns:       benchLength,
			Threads:     threads,
			ImageWidth:  512,
			ImageHeight: 512,
		}

		name := fmt.Sprintf("%dx%dx%d-%d", p.ImageWidth, p.ImageHeight, p.Turns, p.Threads)

		b.Run(name, func(b *testing.B) {
			// Disable program output (optional)
			os.Stdout = nil

			for i := 0; i < b.N; i++ {
				events := gol.EventInterface{}

				// Start the simulation
				go gol.Run(p, &events, nil)

				for {
					ev := events.Get()
					if ev == nil {
						// Nothing yet – avoid tight busy-wait
						time.Sleep(time.Microsecond)
						continue
					}

					sc, ok := ev.(gol.StateChange)

					// Mark this event as consumed so setEvent can send the next one
					events.Set(nil)

					if ok && sc.NewState == gol.Quitting {
						break
					}
				}
			}
		})
	}
}
