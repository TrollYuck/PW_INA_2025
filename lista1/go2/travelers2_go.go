package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

const (
	// Travelers moving on the board
	NrOfTravelers = 15

	MinSteps = 10
	MaxSteps = 100

	MinDelay = 10 * time.Millisecond
	MaxDelay = 50 * time.Millisecond

	// 2D Board with torus topology
	BoardWidth  = 15
	BoardHeight = 15
)

// Position on the board
type Position struct {
	X, Y int
}

// Trace of a traveler at one moment
type Trace struct {
	TimeStamp time.Duration
	Id        int
	Position  Position
	Symbol    rune
}

// A cell that can be occupied/free and responds to requests
type Cell struct {
	request chan chan bool
	occupy  chan struct{}
	free    chan struct{}
}

func NewCell() *Cell {
	c := &Cell{
		request: make(chan chan bool),
		occupy:  make(chan struct{}),
		free:    make(chan struct{}),
	}
	go c.run()
	return c
}

func (c *Cell) run() {
	occupied := false
	for {
		select {
		case respCh := <-c.request:
			respCh <- !occupied
		case <-c.occupy:
			occupied = true
		case <-c.free:
			occupied = false
		}
	}
}

func (c *Cell) Request() bool {
	respCh := make(chan bool)
	c.request <- respCh
	return <-respCh
}

func (c *Cell) Occupy() {
	c.occupy <- struct{}{}
}

func (c *Cell) Free() {
	c.free <- struct{}{}
}

// Message sent from a traveler to printer
type TracesSequence struct {
	Id     int
	Traces []Trace
}

func printer(ch <-chan TracesSequence, wg *sync.WaitGroup) {
	defer wg.Done()
	for i := 0; i < NrOfTravelers; i++ {
		seq := <-ch
		for _, t := range seq.Traces {
			// Format timestamp similarly to Ada's Duration'Image
			fmt.Printf("%8.6f %2d %2d %2d %c\n",
				t.TimeStamp.Seconds(), seq.Id, t.Position.X, t.Position.Y, t.Symbol)
		}
	}
}

func traveler(id int, sym rune, cells [][]*Cell, start time.Time,
	startCh <-chan struct{}, outCh chan<- TracesSequence, seed int64) {

	// Per-traveler RNG
	r := rand.New(rand.NewSource(seed))

	// INIT phase
	var pos Position
	for {
		pos = Position{
			X: r.Intn(BoardWidth),
			Y: r.Intn(BoardHeight),
		}
		if cells[pos.X][pos.Y].Request() {
			cells[pos.X][pos.Y].Occupy()
			break
		}
		time.Sleep(1 * time.Millisecond)
	}

	steps := MinSteps + r.Intn(MaxSteps-MinSteps+1)

	// Collect traces
	traces := make([]Trace, 0, steps+1)
	record := func(sym rune) {
		traces = append(traces, Trace{
			TimeStamp: time.Since(start),
			Id:        id,
			Position:  pos,
			Symbol:    sym,
		})
	}
	record(sym)

	// WAIT for Start signal
	<-startCh

	// MOVEMENT phase
	for step := 0; step < steps; step++ {
		// Delay before move
		d := MinDelay + time.Duration(r.Float64()*float64(MaxDelay-MinDelay))
		time.Sleep(d)

		// Choose a direction
		newPos := pos
		switch r.Intn(4) {
		case 0:
			newPos.Y = (newPos.Y + BoardHeight - 1) % BoardHeight
		case 1:
			newPos.Y = (newPos.Y + 1) % BoardHeight
		case 2:
			newPos.X = (newPos.X + BoardWidth - 1) % BoardWidth
		case 3:
			newPos.X = (newPos.X + 1) % BoardWidth
		}

		// Try to occupy new cell within MaxDelay
		startAttempt := time.Now()
		stuck := false
		for {
			if cells[newPos.X][newPos.Y].Request() {
				// move
				cells[pos.X][pos.Y].Free()
				cells[newPos.X][newPos.Y].Occupy()
				pos = newPos
				break
			}
			if time.Since(startAttempt) > MaxDelay {
				// stuck: lowercase symbol
				sym = rune(int(sym) + 32)
				stuck = true
				break
			}
			time.Sleep(1 * time.Millisecond)
		}

		record(sym)
		if stuck {
			break
		}
	}

	// Report to printer
	outCh <- TracesSequence{Id: id, Traces: traces}
}

func main() {
	// Global start time
	startTime := time.Now()

	// Initialize board cells
	cells := make([][]*Cell, BoardWidth)
	for x := 0; x < BoardWidth; x++ {
		cells[x] = make([]*Cell, BoardHeight)
		for y := 0; y < BoardHeight; y++ {
			cells[x][y] = NewCell()
		}
	}

	// Channel for traveler reports
	reportCh := make(chan TracesSequence, NrOfTravelers)
	var wg sync.WaitGroup

	// Start printer
	wg.Add(1)
	go printer(reportCh, &wg)

	// Create start signal channel
	startCh := make(chan struct{})

	// Launch travelers (Init)
	for i := 0; i < NrOfTravelers; i++ {
		go traveler(i, rune('A'+i), cells, startTime, startCh, reportCh, time.Now().UnixNano()+int64(i))
	}

	// Signal all travelers to start
	close(startCh)

	// Wait for printer to finish
	wg.Wait()

	// Print board parameters at end
	fmt.Printf("-1 %d %d %d\n", NrOfTravelers, BoardWidth, BoardHeight)
}
