package main

import (
	"fmt"
	"math/rand"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	nrOfProcesses = 2
	minSteps      = 50
	maxSteps      = 100
	minDelay      = 10 * time.Millisecond
	maxDelay      = 50 * time.Millisecond
)

// ProcessState enum
type ProcessState int

const (
	LocalSection ProcessState = iota
	EntryProtocol
	CriticalSection
	ExitProtocol
)

func (ps ProcessState) String() string {
	return [...]string{"LOCAL_SECTION", "ENTRY_PROTOCOL", "CRITICAL_SECTION", "EXIT_PROTOCOL"}[ps]
}

// Board dimensions
var (
	boardWidth  = nrOfProcesses
	boardHeight = int(ExitProtocol) + 1
)

// Timing
var startTime time.Time

// Peterson's Algorithm shared variables
var interested [nrOfProcesses]atomic.Bool

// victim indicates whose turn it is to wait if both are interested.
var victim atomic.Int32

// Position_Type struct
type Position struct {
	X int
	Y int
}

// Trace_Type struct
type Trace struct {
	TimeStamp time.Duration
	ID        int
	Position  Position
	Symbol    rune
}

// Traces_Sequence_Type struct
type TracesSequence struct {
	Last       int
	TraceArray []Trace
}

func printTrace(trace Trace) {
	fmt.Printf("%.9fs %d %d %d %c\n",
		trace.TimeStamp.Seconds(),
		trace.ID,
		trace.Position.X,
		trace.Position.Y,
		trace.Symbol,
	)
}

// Print_Traces
func printTraces(traces TracesSequence) {
	for i := 0; i <= traces.Last; i++ {
		printTrace(traces.TraceArray[i])
	}
}

// reportChan for processes to send traces to the printer goroutine
var reportChan = make(chan TracesSequence, nrOfProcesses)
var wgPrinter sync.WaitGroup

// Printer goroutine
func printerGoroutine() {
	defer wgPrinter.Done()

	for i := 0; i < nrOfProcesses; i++ {
		traces := <-reportChan
		printTraces(traces)
	}

	var stateStrings []string
	for i := LocalSection; i <= ExitProtocol; i++ {
		stateStrings = append(stateStrings, i.String())
	}

	fmt.Fprintf(os.Stdout, "-1 %d %d %d %s;\n",
		nrOfProcesses,
		boardWidth,
		boardHeight,
		strings.Join(stateStrings, ";"),
	)
}

// Process_Info
type ProcessInfo struct {
	ID       int
	Symbol   rune
	Position Position
}

var wgProcesses sync.WaitGroup

// processGoroutine
func processGoroutine(id int, seed int64, symbol rune, startSignal <-chan struct{}) {
	defer wgProcesses.Done()

	r := rand.New(rand.NewSource(seed))

	process := ProcessInfo{
		ID:     id,
		Symbol: symbol,
		Position: Position{
			X: id,
			Y: int(LocalSection),
		},
	}

	traces := TracesSequence{
		Last:       -1,
		TraceArray: make([]Trace, maxSteps+1), // Max possible traces + initial
	}

	me := id
	other := 0
	if me == 0 {
		other = 1
	} else {
		other = 0
	}

	storeTrace := func(currentTime time.Duration) {
		traces.Last++
		if traces.Last < len(traces.TraceArray) {
			traces.TraceArray[traces.Last] = Trace{
				TimeStamp: currentTime,
				ID:        process.ID,
				Position:  process.Position,
				Symbol:    process.Symbol,
			}
		} else {
			fmt.Fprintf(os.Stderr, "Warning: Trace array overflow for process %d\n", process.ID)
		}
	}

	changeState := func(state ProcessState) {
		currentTime := time.Since(startTime)
		process.Position.Y = int(state)
		storeTrace(currentTime)
	}

	initialTime := time.Since(startTime)
	storeTrace(initialTime)

	totalStateChangesTarget := minSteps + int(float64(maxSteps-minSteps)*r.Float64())
	numberOfCycles := totalStateChangesTarget / 4
	if numberOfCycles == 0 && totalStateChangesTarget > 0 {
		numberOfCycles = 1
	}

	<-startSignal

	for range numberOfCycles {
		// LOCAL_SECTION
		delayDuration := minDelay + time.Duration(float64(maxDelay-minDelay)*r.Float64())
		time.Sleep(delayDuration)

		changeState(EntryProtocol)
		// Peterson's Entry Protocol
		interested[me].Store(true)
		victim.Store(int32(me))
		for interested[other].Load() && victim.Load() == int32(me) {
			time.Sleep(1 * time.Microsecond)
		}

		changeState(CriticalSection)
		// CRITICAL_SECTION
		delayDuration = minDelay + time.Duration(float64(maxDelay-minDelay)*r.Float64())
		time.Sleep(delayDuration)

		changeState(ExitProtocol)
		// Peterson's Exit Protocol
		interested[me].Store(false)

		changeState(LocalSection) // Back to local section
	}

	finalTraces := TracesSequence{
		Last:       traces.Last,
		TraceArray: traces.TraceArray[:traces.Last+1],
	}
	reportChan <- finalTraces
}

func main() {
	startTime = time.Now()

	seeds := make([]int64, nrOfProcesses)
	for i := range nrOfProcesses {
		seeds[i] = time.Now().UnixNano() + int64(i*100)
	}

	// Start Printer goroutine
	wgPrinter.Add(1)
	go printerGoroutine()

	// Create start signals for processes
	startSignals := make([]chan struct{}, nrOfProcesses)
	for i := range nrOfProcesses {
		startSignals[i] = make(chan struct{})
	}

	// Start Process goroutines
	currentSymbol := 'A'
	for i := range nrOfProcesses {
		wgProcesses.Add(1)
		go processGoroutine(i, seeds[i], currentSymbol, startSignals[i])
		currentSymbol++
	}

	// Send start signals to all process goroutines
	for i := range nrOfProcesses {
		close(startSignals[i]) // Closing channel broadcasts signal
	}

	wgProcesses.Wait()

	wgPrinter.Wait()
}
