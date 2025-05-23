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
	// Wild traveler parameters
	NrOfWildSpawns  = 10 // total wild spawns
	WildMinLifespan = 500 * time.Millisecond
	WildMaxLifespan = 2000 * time.Millisecond

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
// Id < NrOfTravelers => normal; Id >= NrOfTravelers => wild
// Symbol '0'-'9' for wild, 'A'+i or lowercase for normal
// On wild disappearance, Position = (BoardWidth, BoardHeight)
type Trace struct {
	TimeStamp time.Duration
	Id        int
	Position  Position
	Symbol    rune
}

// A cell that can be occupied/free and responds to requests
// Tracks occupant type: 0 free, 1 normal, 2 wild
// For wild, also holds a channel to request it to move
type Cell struct {
	reqCh    chan chan requestResult
	occupyCh chan occupant
	freeCh   chan struct{}
}

type requestResult struct {
	CanOccupy    bool
	OccupantType int
	WildMoveReq  chan chan bool
}

type occupant struct {
	Typ         int            // 1 = normal, 2 = wild
	WildMoveReq chan chan bool // only for wild
}

func NewCell() *Cell {
	c := &Cell{
		reqCh:    make(chan chan requestResult),
		occupyCh: make(chan occupant),
		freeCh:   make(chan struct{}),
	}
	go c.run()
	return c
}

func (c *Cell) run() {
	occupiedType := 0
	var wildReq chan chan bool
	for {
		select {
		case resp := <-c.reqCh:
			resp <- requestResult{CanOccupy: occupiedType == 0, OccupantType: occupiedType, WildMoveReq: wildReq}
		case occ := <-c.occupyCh:
			occupiedType = occ.Typ
			wildReq = occ.WildMoveReq
		case <-c.freeCh:
			occupiedType = 0
			wildReq = nil
		}
	}
}

// Request checks if cell is free or occupied
// Returns (canOccupy bool, occupantType int, wildMoveReq channel for relocating wild)
func (c *Cell) Request() (bool, int, chan chan bool) {
	respCh := make(chan requestResult)
	c.reqCh <- respCh
	res := <-respCh
	return res.CanOccupy, res.OccupantType, res.WildMoveReq
}

// Occupy marks cell occupied by normal traveler
func (c *Cell) Occupy() {
	c.occupyCh <- occupant{Typ: 1}
}

// OccupyWild marks cell occupied by wild traveler, providing its move request channel
func (c *Cell) OccupyWild(moveReq chan chan bool) {
	c.occupyCh <- occupant{Typ: 2, WildMoveReq: moveReq}
}

// Free marks cell free
func (c *Cell) Free() {
	c.freeCh <- struct{}{}
}

// Message sent from a traveler to printer
// Both normal and wild send TracesSequence
// Normal IDs: 0..NrOfTravelers-1; Wild: NrOfTravelers.. onwards
// Once all are done, printer exits after total

type TracesSequence struct {
	Id     int
	Traces []Trace
}

func printer(ch <-chan TracesSequence, total int, wg *sync.WaitGroup) {
	defer wg.Done()
	for i := 0; i < total; i++ {
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

	r := rand.New(rand.NewSource(seed))

	// INIT phase
	var pos Position
	for {
		pos = Position{X: r.Intn(BoardWidth), Y: r.Intn(BoardHeight)}
		ok, _, _ := cells[pos.X][pos.Y].Request()
		if ok {
			cells[pos.X][pos.Y].Occupy()
			break
		}
		time.Sleep(1 * time.Millisecond)
	}

	steps := MinSteps + r.Intn(MaxSteps-MinSteps+1)

	traces := make([]Trace, 0, steps+1)
	record := func(sym rune) {
		traces = append(traces, Trace{TimeStamp: time.Since(start), Id: id, Position: pos, Symbol: sym})
	}
	record(sym)

	<-startCh

	for step := 0; step < steps; step++ {
		d := MinDelay + time.Duration(r.Float64()*float64(MaxDelay-MinDelay))
		time.Sleep(d)

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

		startAttempt := time.Now()
		stuck := false
		for {
			ok, typ, wildCh := cells[newPos.X][newPos.Y].Request()
			if ok {
				cells[pos.X][pos.Y].Free()
				cells[newPos.X][newPos.Y].Occupy()
				pos = newPos
				break
			} else if typ == 2 {
				// occupied by wild: request relocation
				response := make(chan bool)
				wildCh <- response
				if <-response {
					continue
				}
				// wild couldn't move: choose new direction
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
			} else if time.Since(startAttempt) > MaxDelay {
				sym = rune(int(sym) + 32)
				stuck = true
				break
			} else {
				time.Sleep(1 * time.Millisecond)
			}
		}

		record(sym)
		if stuck {
			break
		}
	}

	outCh <- TracesSequence{Id: id, Traces: traces}
}

func wildTraveler(id int, cells [][]*Cell, start time.Time, outCh chan<- TracesSequence, seed int64) {
	r := rand.New(rand.NewSource(seed))

	var pos Position
	for {
		pos = Position{X: r.Intn(BoardWidth), Y: r.Intn(BoardHeight)}
		if ok, _, _ := cells[pos.X][pos.Y].Request(); ok {
			break
		}
	}

	moveReq := make(chan chan bool)
	cells[pos.X][pos.Y].OccupyWild(moveReq)

	symbol := rune('0' + r.Intn(10))
	traces := []Trace{{TimeStamp: time.Since(start), Id: id, Position: pos, Symbol: symbol}}

	lifespan := WildMinLifespan + time.Duration(r.Float64()*float64(WildMaxLifespan-WildMinLifespan))
	end := time.After(lifespan)

	for {
		select {
		case respCh := <-moveReq:
			moved := false
			// try neighbor cells
			for d := 0; d < 4; d++ {
				temp := pos
				switch d {
				case 0:
					temp.X = (temp.X + 1) % BoardWidth
				case 1:
					temp.X = (temp.X + BoardWidth - 1) % BoardWidth
				case 2:
					temp.Y = (temp.Y + 1) % BoardHeight
				case 3:
					temp.Y = (temp.Y + BoardHeight - 1) % BoardHeight
				}
				if ok, _, _ := cells[temp.X][temp.Y].Request(); ok {
					cells[pos.X][pos.Y].Free()
					cells[temp.X][temp.Y].OccupyWild(moveReq)
					pos = temp
					moved = true
					break
				}
			}
			respCh <- moved
			traces = append(traces, Trace{TimeStamp: time.Since(start), Id: id, Position: pos, Symbol: symbol})
		case <-end:
			cells[pos.X][pos.Y].Free()
			// disappearance
			traces = append(traces, Trace{TimeStamp: time.Since(start), Id: id, Position: Position{BoardWidth, BoardHeight}, Symbol: symbol})
			outCh <- TracesSequence{Id: id, Traces: traces}
			return
		}
	}
}

func main() {
	startTime := time.Now()

	// initialize board
	cells := make([][]*Cell, BoardWidth)
	for x := 0; x < BoardWidth; x++ {
		cells[x] = make([]*Cell, BoardHeight)
		for y := 0; y < BoardHeight; y++ {
			cells[x][y] = NewCell()
		}
	}

	reportCh := make(chan TracesSequence, NrOfTravelers+NrOfWildSpawns)
	var wg sync.WaitGroup
	wg.Add(1)
	go printer(reportCh, NrOfTravelers+NrOfWildSpawns, &wg)

	startCh := make(chan struct{})

	// launch normal travelers
	for i := 0; i < NrOfTravelers; i++ {
		go traveler(i, rune('A'+i), cells, startTime, startCh, reportCh, time.Now().UnixNano()+int64(i))
	}
	// launch wild travelers
	for i := 0; i < NrOfWildSpawns; i++ {
		go wildTraveler(NrOfTravelers+i, cells, startTime, reportCh, time.Now().UnixNano()+int64(i)*12345)
	}

	// start normals
	close(startCh)

	wg.Wait()
	fmt.Printf("-1 %d %d %d\n", NrOfTravelers, BoardWidth, BoardHeight)
}
