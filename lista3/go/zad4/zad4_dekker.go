package main

import (
	"fmt"
	"math/rand"
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

var boardWidth = nrOfProcesses
var boardHeight = int(ExitProtocol) + 1

var startTime time.Time

// Dekker's Algorithm Shared Variables
// want[i] is 1 if process i wants to enter, 0 otherwise.
var want [nrOfProcesses]int32

// turn is the ID of the process whose turn it is.
var turn int32 // 0 or 1

type Position struct {
	X, Y int
}

type Trace struct {
	TimeStamp time.Duration
	ID        int
	Position  Position
	Symbol    rune
}

func printTrace(trace Trace) {
	seconds := trace.TimeStamp.Seconds()
	fmt.Printf(" %.9f %d %d %d  %c\n",
		seconds,
		trace.ID,
		trace.Position.X,
		trace.Position.Y,
		trace.Symbol,
	)
}

// printTraces prints all traces for a process.
func printTraces(traces []Trace) {
	for _, t := range traces {
		printTrace(t)
	}
}

// printerTask collects traces from all processes and prints them, then prints the summary line.
func printerTask(traceChan <-chan []Trace, wg *sync.WaitGroup) {
	defer wg.Done()

	allProcessTraces := make([][]Trace, nrOfProcesses)
	for range nrOfProcesses {
		processTraces := <-traceChan // Receive traces from a process
		if len(processTraces) > 0 {
			processID := processTraces[0].ID
			if processID >= 0 && processID < nrOfProcesses {
				allProcessTraces[processID] = processTraces
			}
		}
	}

	for i := range nrOfProcesses {
		if allProcessTraces[i] != nil {
			printTraces(allProcessTraces[i])
		}
	}

	var stateLabels []string
	for i := LocalSection; i <= ExitProtocol; i++ {
		stateLabels = append(stateLabels, i.String())
	}

	fmt.Printf("-1 %d %d %d %sEXTRA_LABEL;\n",
		nrOfProcesses,
		boardWidth,
		boardHeight,
		strings.Join(stateLabels, ";")+";",
	)
}

type ProcessData struct {
	ID       int
	Symbol   rune
	Position Position
}

// processTask simulates a single process executing Dekker's algorithm.
func processTask(id int, seed int64, symbol rune, traceChan chan<- []Trace, wg *sync.WaitGroup) {
	defer wg.Done()

	r := rand.New(rand.NewSource(seed))
	process := ProcessData{
		ID:     id,
		Symbol: symbol,
		Position: Position{
			X: id,
			Y: int(LocalSection),
		},
	}

	var traces []Trace
	var currentTimeStamp time.Duration

	me := int32(id)
	other := int32(1 - me)

	storeTrace := func() {
		traces = append(traces, Trace{
			TimeStamp: currentTimeStamp,
			ID:        process.ID,
			Position:  process.Position,
			Symbol:    process.Symbol,
		})
	}

	changeState := func(state ProcessState) {
		currentTimeStamp = time.Since(startTime)
		process.Position.Y = int(state)
		storeTrace()
	}

	currentTimeStamp = time.Since(startTime)
	storeTrace()

	baseNrOfSteps := minSteps + r.Intn(maxSteps-minSteps+1)
	loopIterations := baseNrOfSteps / 4

	for step := 0; step < loopIterations; step++ {
		// LOCAL_SECTION
		delayNs := float64(minDelay.Nanoseconds()) + r.Float64()*float64((maxDelay-minDelay).Nanoseconds())
		time.Sleep(time.Duration(delayNs))

		changeState(EntryProtocol)

		// Dekker's Entry Protocol for Process `me`
		atomic.StoreInt32(&want[me], 1) // I want to enter (true)
		for atomic.LoadInt32(&want[other]) == 1 {
			if atomic.LoadInt32(&turn) == other {
				atomic.StoreInt32(&want[me], 0)
				for atomic.LoadInt32(&turn) == other {
				}
				atomic.StoreInt32(&want[me], 1) // Re-assert my intention, it's my turn now (true)
			} else {

			}
		}

		changeState(CriticalSection)

		// CRITICAL_SECTION
		delayNs = float64(minDelay.Nanoseconds()) + r.Float64()*float64((maxDelay-minDelay).Nanoseconds())
		time.Sleep(time.Duration(delayNs))

		changeState(ExitProtocol)

		// Dekker's Exit Protocol
		atomic.StoreInt32(&turn, other)
		atomic.StoreInt32(&want[me], 0)

		changeState(LocalSection)
	}

	traceChan <- traces
}

func main() {
	startTime = time.Now()

	var seeds [nrOfProcesses]int64
	for i := range nrOfProcesses {
		seeds[i] = time.Now().UnixNano() + int64(i*100)
	}

	traceChan := make(chan []Trace, nrOfProcesses)

	var processWg sync.WaitGroup
	var printerWg sync.WaitGroup

	printerWg.Add(1)
	go printerTask(traceChan, &printerWg)

	currentSymbol := 'A'
	for i := range nrOfProcesses {
		processWg.Add(1)
		go processTask(i, seeds[i], currentSymbol, traceChan, &processWg)
		currentSymbol++
	}

	processWg.Wait()
	close(traceChan)
	printerWg.Wait()
}
