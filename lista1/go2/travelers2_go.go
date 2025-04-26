package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// Travelers moving on the board
const NrOfTravelers int = 15

var cellLocks [][]sync.Mutex

const (
	MinSteps int = 10
	MaxSteps int = 100
)

const (
	MinDelay time.Duration = 10 * time.Millisecond
	MaxDelay time.Duration = 50 * time.Millisecond
)

// 2D Board with torus topology
const (
	BoardWidth  int = 15
	BoardHeight int = 15
)

// Timing
var StartTime time.Time = time.Now() // global starting time

// Random seeds for the tasks' random number generators
var seeds [NrOfTravelers]int

func init() {
	// Seed the random number generator
	//rand.Seed(time.Now().UnixNano())

	// Generate random seeds for each traveler
	for i := 0; i < NrOfTravelers; i++ {
		seeds[i] = rand.Int() // Generate a random integer
	}

	// Initialize cell locks
	cellLocks = make([][]sync.Mutex, BoardWidth)
	for i := range cellLocks {
		cellLocks[i] = make([]sync.Mutex, BoardHeight)
	}
}

// PositionType represents a position on the board
type PositionType struct {
	X int // X coordinate, range: 0 to BoardWidth - 1
	Y int // Y coordinate, range: 0 to BoardHeight - 1
}

// MoveDown moves the position down on the board (with wrap-around)
func (p PositionType) MoveDown() PositionType {
	return PositionType{
		X: p.X,
		Y: (p.Y + 1) % BoardHeight,
	}
}

// MoveUp moves the position up on the board (with wrap-around)
func (p PositionType) MoveUp() PositionType {
	return PositionType{
		X: p.X,
		Y: (p.Y + BoardHeight - 1) % BoardHeight,
	}
}

// MoveRight moves the position right on the board (with wrap-around)
func (p PositionType) MoveRight() PositionType {
	return PositionType{
		X: (p.X + 1) % BoardWidth,
		Y: p.Y,
	}
}

// MoveLeft moves the position left on the board (with wrap-around)
func (p PositionType) MoveLeft() PositionType {
	return PositionType{
		X: (p.X + BoardWidth - 1) % BoardWidth,
		Y: p.Y,
	}
}

// TraceType represents a trace of a traveler
type TraceType struct {
	TimeStamp time.Duration // Time stamp of the trace
	Id        int           // Traveler ID
	Position  PositionType  // Position of the traveler
	Symbol    rune          // Symbol representing the traveler
}

// TracesSequenceType represents a sequence of traces
type TracesSequenceType struct {
	Last       int         // Index of the last trace
	TraceArray []TraceType // Array of traces
}

// PrintTrace prints a single trace
func PrintTrace(trace TraceType) {
	fmt.Printf("%v %d %d %d %c\n",
		trace.TimeStamp, trace.Id, trace.Position.X, trace.Position.Y, trace.Symbol)
}

// PrintTraces prints all traces in a sequence
func PrintTraces(traces TracesSequenceType) {
	for i := 0; i <= traces.Last; i++ {
		PrintTrace(traces.TraceArray[i])
	}
}

// Printer collects and prints reports of traces
type Printer struct {
	wg      sync.WaitGroup
	reports chan TracesSequenceType // Channel for trace reports
}

func NewPrinter() *Printer {
	return &Printer{
		reports: make(chan TracesSequenceType, NrOfTravelers),
	}
}

func (p *Printer) Start() {
	go func() {
		for traces := range p.reports {
			PrintTraces(traces)
			p.wg.Done()
		}
	}()
}

// Report sends traces to the Printer's channel
func (p *Printer) Report(traces TracesSequenceType) {
	p.reports <- traces
}

// Stop closes the Printer's channel
func (p *Printer) Stop() {
	close(p.reports)
}

// TravelerTask represents a traveler task
type TravelerTask struct {
	Id        int
	Seed      int
	Symbol    rune
	Position  PositionType
	Steps     int
	Traces    TracesSequenceType
	Generator *rand.Rand
	Printer   *Printer
}

// Init initializes the traveler task
func (t *TravelerTask) Init(id int, seed int, symbol rune) {
	t.Id = id
	t.Seed = seed
	t.Symbol = symbol
	t.Generator = rand.New(rand.NewSource(int64(seed)))
	t.Position = PositionType{
		X: t.Generator.Intn(BoardWidth),
		Y: t.Generator.Intn(BoardHeight),
	}
	t.Traces = TracesSequenceType{
		Last:       -1,
		TraceArray: make([]TraceType, MaxSteps),
	}
	t.StoreTrace()
	t.Steps = MinSteps + t.Generator.Intn(MaxSteps-MinSteps)
}

// StoreTrace stores the current trace
func (t *TravelerTask) StoreTrace() {
	if t.Traces.Last+1 >= len(t.Traces.TraceArray) {
		// Prevent out-of-bounds access by stopping trace storage
		fmt.Printf("Warning: Trace array full for traveler %d\n", t.Id)
		return
	}

	t.Traces.Last++
	t.Traces.TraceArray[t.Traces.Last] = TraceType{
		TimeStamp: time.Since(StartTime),
		Id:        t.Id,
		Position:  t.Position,
		Symbol:    t.Symbol,
	}
}

// MakeStep makes a random step
func (t *TravelerTask) MakeStep() bool {
	var newPos PositionType
	switch t.Generator.Intn(4) {
	case 0:
		newPos = t.Position.MoveUp()
	case 1:
		newPos = t.Position.MoveDown()
	case 2:
		newPos = t.Position.MoveLeft()
	case 3:
		newPos = t.Position.MoveRight()
	}

	locked := make(chan bool, 1)
	go func() {
		cellLocks[newPos.X][newPos.Y].Lock()
		locked <- true
	}()
	select {
	case <-locked:
		// Successfully locked the target cell
		cellLocks[t.Position.X][t.Position.Y].Unlock()
		t.Position = newPos
		t.StoreTrace()
		return true
	case <-time.After(MaxDelay):
		// Timeout: deadlock detected
		t.Symbol = rune(t.Symbol + 32) // Convert symbol to lowercase
		t.StoreTrace()                 // Store the final trace
		return false
	}
}

// Start starts the traveler task
func (t *TravelerTask) Start() {
	for i := 0; i < t.Steps; i++ {
		time.Sleep(MinDelay + time.Duration(t.Generator.Int63n(int64(MaxDelay-MinDelay))))
		if !t.MakeStep() {
			// Traveler encountered a deadlock
			break
		}
	}
	t.Printer.Report(t.Traces)
	cellLocks[t.Position.X][t.Position.Y].Unlock() // Unlock the final position
}

func main() {
	// Initialize the mutex grid
	for i := range cellLocks {
		for j := range cellLocks[i] {
			cellLocks[i][j] = sync.Mutex{}
		}
	}

	printer := NewPrinter()

	printer.Start()

	printer.wg.Add(NrOfTravelers)

	travelers := make([]*TravelerTask, NrOfTravelers)
	symbol := 'A'

	// Initialize travelers
	for i := 0; i < NrOfTravelers; i++ {
		travelers[i] = &TravelerTask{
			Printer: printer,
		}
		travelers[i].Init(i, seeds[i], symbol)
		symbol++
		cellLocks[travelers[i].Position.X][travelers[i].Position.Y].Lock() // Lock initial position
	}

	// Start travelers
	for _, traveler := range travelers {
		go traveler.Start()
	}

	// Wait for all travelers to finish
	printer.wg.Wait()

	printer.Stop()

	// Print board parameters for display script
	fmt.Printf("-1 %d %d %d\n", NrOfTravelers, BoardWidth, BoardHeight)
}
