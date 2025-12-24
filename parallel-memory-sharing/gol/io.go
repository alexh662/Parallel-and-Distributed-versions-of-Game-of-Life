package gol

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"

	"uk.ac.bris.cs/gameoflife/util"
)

type IoInterface struct {
	mu           sync.Mutex
	cond         *sync.Cond
	events       chan Event
	command      ioCommand
	idle         boolInterface
	filename     stringInterface
	buffer       []byte
	request      bool
	shuttingDown bool
}

// ioState is the internal ioState of the io goroutine.
type ioState struct {
	params      Params
	ioInterface *IoInterface
}

// ioCommand allows requesting behaviour from the io (pgm) goroutine.
type ioCommand uint8

// This is a way of creating enums in Go.
// It will evaluate to:
//
//	ioOutput 	= 0
//	ioInput 	= 1
//	ioCheckIdle = 2
const (
	ioOutput ioCommand = iota
	ioInput
)

// writePgmImage receives an array of bytes and writes it to a pgm file.
func (io *ioState) writePgmImage() {
	_ = os.Mkdir("out", os.ModePerm)

	// Request a filename from the distributor.
	filename := io.ioInterface.filename.get()

	file, ioError := os.Create("out/" + filename + ".pgm")
	util.Check(ioError)
	defer file.Close()

	_, _ = file.WriteString("P5\n")
	//_, _ = file.WriteString("# PGM file writer by pnmmodules (https://github.com/owainkenwayucl/pnmmodules).\n")
	_, _ = file.WriteString(strconv.Itoa(io.params.ImageWidth))
	_, _ = file.WriteString(" ")
	_, _ = file.WriteString(strconv.Itoa(io.params.ImageHeight))
	_, _ = file.WriteString("\n")
	_, _ = file.WriteString(strconv.Itoa(255))
	_, _ = file.WriteString("\n")

	io.ioInterface.mu.Lock()
	buf := make([]byte, len(io.ioInterface.buffer))
	copy(buf, io.ioInterface.buffer)
	io.ioInterface.mu.Unlock()

	_, ioError = file.Write(buf)
	util.Check(ioError)

	ioError = file.Sync()
	util.Check(ioError)

	log.Printf("[IO] File %v.pgm output done", filename)
}

// readPgmImage opens a pgm file and sends its data as an array of bytes.
func (io *ioState) readPgmImage() {

	// Request a filename from the distributor.
	filename := io.ioInterface.filename.get()

	data, ioError := os.ReadFile("images/" + filename + ".pgm")
	util.Check(ioError)

	fields := strings.Fields(string(data))

	if fields[0] != "P5" {
		panic(fmt.Sprintf("[IO] %v %v is not a pgm file", util.Red("ERROR"), filename))
	}

	width, _ := strconv.Atoi(fields[1])
	if width != io.params.ImageWidth {
		panic(fmt.Sprintf("[IO] %v Incorrect pgm width", util.Red("ERROR")))
	}

	height, _ := strconv.Atoi(fields[2])
	if height != io.params.ImageHeight {
		panic(fmt.Sprintf("[IO] %v Incorrect pgm height", util.Red("ERROR")))
	}

	maxval, _ := strconv.Atoi(fields[3])
	if maxval != 255 {
		panic(fmt.Sprintf("[IO] %v Incorrect pgm maxval/bit depth", util.Red("ERROR")))
	}

	image := []byte(fields[4])

	io.ioInterface.mu.Lock()
	io.ioInterface.buffer = make([]byte, len(image))
	copy(io.ioInterface.buffer, image)
	io.ioInterface.mu.Unlock()

	log.Printf("[IO] File %v.pgm input done", filename)
}

// startIo should be the entrypoint of the io goroutine.
func startIo(p Params, i *IoInterface) {
	io := ioState{
		params:      p,
		ioInterface: i,
	}

	for {
		i.mu.Lock()
		for !i.request && !i.shuttingDown {
			i.cond.Wait()
		}
		if i.shuttingDown {
			i.mu.Unlock()
			return
		}
		// Block and wait for requests from the distributor
		command := i.command
		i.request = false
		i.mu.Unlock()
		switch command {
		case ioInput:
			io.readPgmImage()
		case ioOutput:
			io.writePgmImage()
		}
		i.idle.setTrue()
	}
}
