package main

import (
	"fmt"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

const (
	nrOfProcesses = 15
	minSteps      = 50
	maxSteps      = 100
	minDelayMs    = 10 // 0.01s
	maxDelayMs    = 50 // 0.05s
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

// Board dimensions
const (
	boardWidth  = nrOfProcesses
	boardHeight = int(ExitProtocol) + 1
)

// Timing
var startTime time.Time

// Shared variables for Bakery Algorithm
var (
	choosing []int32 // Use int32 for atomic operations (0 or 1)
	number   []int64 // Use int64 for ticket numbers
)

// Max_Ticket_Tracker
var storedMaxTicket int64 // Accessed atomically

func updateOverallMax(ticketValue int64) {
	for {
		oldMax := atomic.LoadInt64(&storedMaxTicket)
		if ticketValue > oldMax {
			if atomic.CompareAndSwapInt64(&storedMaxTicket, oldMax, ticketValue) {
				return
			}
		} else {
			return
		}
	}
}

func getOverallMax() int64 {
	return atomic.LoadInt64(&storedMaxTicket)
}

// Position_Type
type Position struct {
	X int
	Y int
}

// Trace_Type
type Trace struct {
	TimeStamp time.Duration
	Id        int
	Position  Position
	Symbol    rune
}

// Traces_Sequence_Type
type TracesSequence []Trace

// Global random number generator for initial seed generation
var globalRand = rand.New(rand.NewSource(time.Now().UnixNano()))

func printTrace(trace Trace) {
	fmt.Printf("%.9f %d %d %d %c\n",
		trace.TimeStamp.Seconds(),
		trace.Id,
		trace.Position.X,
		trace.Position.Y,
		trace.Symbol,
	)
}

func printTraces(traces TracesSequence) {
	for _, trace := range traces {
		printTrace(trace)
	}
}

// Printer task
func printerTask(reportChan <-chan TracesSequence, wg *sync.WaitGroup) {
	defer wg.Done()

	// Loop nrOfProcesses times to receive and print traces
	for i := 0; i < nrOfProcesses; i++ {
		traces := <-reportChan
		printTraces(traces)
		// Optionally, you could collect them if needed for other purposes:
		// allTraces = append(allTraces, traces)
	}

	// Final parameter printing
	fmt.Fprintf(os.Stdout, "-1 %d %d %d ", nrOfProcesses, boardWidth, boardHeight)
	for i := ProcessState(0); i <= ExitProtocol; i++ {
		fmt.Fprintf(os.Stdout, "%s;", i.String())
	}
	fmt.Fprintf(os.Stdout, "MAX_TICKET=%d;\n", getOverallMax())
}

// Helper Max function for Bakery Algorithm
func bakeryMax() int64 {
	currentMax := int64(0)
	for i := 0; i < nrOfProcesses; i++ {
		num_i := atomic.LoadInt64(&number[i])
		if num_i > currentMax {
			currentMax = num_i
		}
	}
	return currentMax
}

// Process_Type
type processInfo struct {
	Id       int
	Symbol   rune
	Position Position
}

// Process_Task_Type
func processTask(
	id int,
	seed int64,
	symbol rune,
	wg *sync.WaitGroup,
	reportChan chan<- TracesSequence,
	startSignal <-chan struct{}, // To synchronize start
) {
	defer wg.Done()

	localRand := rand.New(rand.NewSource(seed))
	process := processInfo{
		Id:     id,
		Symbol: symbol,
		Position: Position{
			X: id,
			Y: int(LocalSection),
		},
	}

	traces := make(TracesSequence, 0, maxSteps+1) // Pre-allocate
	var myHighestTicket int64 = 0

	storeTrace := func(state ProcessState) {
		ts := time.Since(startTime)
		process.Position.Y = int(state)
		traces = append(traces, Trace{
			TimeStamp: ts,
			Id:        process.Id,
			Position:  process.Position,
			Symbol:    process.Symbol,
		})
	}

	// Initial trace
	storeTrace(LocalSection)

	// Number of steps for this process
	nrOfSteps := minSteps + localRand.Intn(maxSteps-minSteps+1)

	// Wait for global start signal
	<-startSignal

	// Main loop
	// Ada: for Step in 0 .. Nr_of_Steps/4 - 1  loop
	// This means Nr_of_Steps/4 iterations.
	iterations := nrOfSteps / 4
	if iterations == 0 && nrOfSteps > 0 {
		iterations = 1
	}

	for range iterations {
		// LOCAL_SECTION
		delay := minDelayMs + localRand.Intn(maxDelayMs-minDelayMs+1)
		time.Sleep(time.Duration(delay) * time.Millisecond)

		// ENTRY_PROTOCOL
		storeTrace(EntryProtocol)
		atomic.StoreInt32(&choosing[process.Id], 1)

		newTicket := 1 + bakeryMax()
		atomic.StoreInt64(&number[process.Id], newTicket)
		if newTicket > myHighestTicket {
			myHighestTicket = newTicket
		}
		atomic.StoreInt32(&choosing[process.Id], 0)

		for j := range nrOfProcesses {
			if j == process.Id {
				continue
			}
			// Wait for choosing[j] to be 0
			for atomic.LoadInt32(&choosing[j]) == 1 {
				time.Sleep(1 * time.Microsecond) // Small sleep
			}
			// Wait for number[j] to be 0, or for (number[id], id) < (number[j], j)
			for {
				numJ := atomic.LoadInt64(&number[j])
				numID := atomic.LoadInt64(&number[process.Id])

				if numJ == 0 || (numID < numJ) || (numID == numJ && process.Id < j) {
					break
				}
				time.Sleep(1 * time.Microsecond) // Small sleep
			}
		}

		// CRITICAL_SECTION
		storeTrace(CriticalSection)
		delay = minDelayMs + localRand.Intn(maxDelayMs-minDelayMs+1)
		time.Sleep(time.Duration(delay) * time.Millisecond)

		// EXIT_PROTOCOL
		storeTrace(ExitProtocol)
		atomic.StoreInt64(&number[process.Id], 0)

		// Back to LOCAL_SECTION for the next iteration
		storeTrace(LocalSection)
	}

	updateOverallMax(myHighestTicket)
	reportChan <- traces
}

func main() {
	startTime = time.Now()

	choosing = make([]int32, nrOfProcesses)
	number = make([]int64, nrOfProcesses)

	var wgProcesses sync.WaitGroup
	var wgPrinter sync.WaitGroup

	reportChan := make(chan TracesSequence) // Unbuffered to ensure printer processes one by one

	// Start Printer task
	wgPrinter.Add(1)
	go printerTask(reportChan, &wgPrinter) // Call printerTask as a goroutine

	startSignal := make(chan struct{}) // Channel to synchronize the start of process tasks

	// Init and Start Process Tasks
	for i := 0; i < nrOfProcesses; i++ {
		wgProcesses.Add(1)
		// Ada: Seeds(I+1) - assuming 1-indexed seeds array.
		// Go: generate a unique seed for each.
		seed := time.Now().UnixNano() + int64(i*100) // Simple unique seed
		symbol := rune('A' + i)
		go processTask(i, seed, symbol, &wgProcesses, reportChan, startSignal)
	}

	// Signal all process tasks to start after they are initialized
	close(startSignal)

	// Wait for all process tasks to complete
	wgProcesses.Wait()

	// All process tasks have sent their reports.
	// Now, wait for the printerTask goroutine to finish processing all reports and printing the footer.
	wgPrinter.Wait()
}
