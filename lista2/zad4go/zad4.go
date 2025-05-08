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

	// Traps ─── TRAP
	NrOfTraps     = 15
	TrapBlockTime = 500 * time.Millisecond

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
	reqCh    chan chan requestResult
	occupyCh chan occupant
	freeCh   chan struct{}

	// Trap flag ─── TRAP
	isTrap bool
	trapId int
}

type requestResult struct {
	CanOccupy    bool
	OccupantType int
	WildMoveReq  chan chan bool
	IsTrap       bool // ─── TRAP
}

type occupant struct {
	Typ         int
	WildMoveReq chan chan bool
}

func NewCell() *Cell {
	c := &Cell{
		reqCh:    make(chan chan requestResult),
		occupyCh: make(chan occupant),
		freeCh:   make(chan struct{}),
		isTrap:   false, // ─── TRAP
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
			resp <- requestResult{
				CanOccupy:    occupiedType == 0,
				OccupantType: occupiedType,
				WildMoveReq:  wildReq,
				IsTrap:       c.isTrap, // ─── TRAP
			}
		case occ := <-c.occupyCh:
			occupiedType = occ.Typ
			wildReq = occ.WildMoveReq
		case <-c.freeCh:
			occupiedType = 0
			wildReq = nil
		}
	}
}

func (c *Cell) Request() (bool, int, chan chan bool, bool) {
	respCh := make(chan requestResult)
	c.reqCh <- respCh
	res := <-respCh
	return res.CanOccupy, res.OccupantType, res.WildMoveReq, res.IsTrap // ─── TRAP
}

func (c *Cell) Occupy() {
	c.occupyCh <- occupant{Typ: 1}
}

func (c *Cell) OccupyWild(moveReq chan chan bool) {
	c.occupyCh <- occupant{Typ: 2, WildMoveReq: moveReq}
}

func (c *Cell) Free() {
	c.freeCh <- struct{}{}
}

type TracesSequence struct {
	Id     int
	Traces []Trace
}

func printer(ch <-chan TracesSequence, total int, wg *sync.WaitGroup) {
	defer wg.Done()
	for i := 0; i < total; i++ {
		seq := <-ch
		for _, t := range seq.Traces {
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
		ok, _, _, isTrap := cells[pos.X][pos.Y].Request()
		if ok && !isTrap { // can't start on trap ─── TRAP
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
			ok, typ, wildCh, isTrap := cells[newPos.X][newPos.Y].Request()
			if ok {
				// stepping into a trap? ─── TRAP
				if isTrap {
					// mark lowercase, block and exit
					sym = rune(int(sym) + 32)
					cells[newPos.X][newPos.Y].Occupy()
					record(sym)
					time.Sleep(TrapBlockTime)
					cells[newPos.X][newPos.Y].Free()
					outCh <- TracesSequence{Id: id, Traces: traces}
					outCh <- TracesSequence{
						Id: cells[newPos.X][newPos.Y].trapId,
						Traces: []Trace{
							{
								TimeStamp: time.Since(start),
								Id:        cells[newPos.X][newPos.Y].trapId, // Same ID here
								Position:  Position{X: newPos.X, Y: newPos.Y},
								Symbol:    '#',
							},
						},
					}
					return
				}
				// normal move
				cells[pos.X][pos.Y].Free()
				cells[newPos.X][newPos.Y].Occupy()
				pos = newPos
				break
			} else if typ == 2 {
				// occupied by wild
				resp := make(chan bool)
				wildCh <- resp
				if <-resp {
					continue
				}
				// choose another direction
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

	// INIT phase, avoid traps ─── TRAP
	var pos Position
	for {
		pos = Position{X: r.Intn(BoardWidth), Y: r.Intn(BoardHeight)}
		ok, _, _, isTrap := cells[pos.X][pos.Y].Request()
		if ok && !isTrap {
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
				ok, _, _, isTrap := cells[temp.X][temp.Y].Request()
				if ok {
					// if trap, symbol "*", block, then exit ─── TRAP
					if isTrap {
						symbol = '*'
						cells[pos.X][pos.Y].Free()
						cells[temp.X][temp.Y].OccupyWild(moveReq)
						pos = temp
						traces = append(traces, Trace{TimeStamp: time.Since(start), Id: id, Position: pos, Symbol: symbol})
						time.Sleep(TrapBlockTime)
						cells[pos.X][pos.Y].Free()
						outCh <- TracesSequence{Id: id, Traces: traces}
						outCh <- TracesSequence{
							Id: cells[temp.X][temp.Y].trapId,
							Traces: []Trace{
								{
									TimeStamp: time.Since(start),
									Id:        cells[temp.X][temp.Y].trapId, // Same ID here
									Position:  Position{X: temp.X, Y: temp.Y},
									Symbol:    '#',
								},
							},
						}
						return
					}
					// normal wild move
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

	// place traps ─── TRAP
	r := rand.New(rand.NewSource(startTime.UnixNano()))
	placed := 0 - NrOfTraps
	for placed < 0 {
		x := r.Intn(BoardWidth)
		y := r.Intn(BoardHeight)
		cell := cells[x][y]
		// only place on empty, non-trap cell
		if can, _, _, isTrap := cell.Request(); can && !isTrap {
			cell.isTrap = true
			cell.trapId = placed
			placed++
			reportCh <- TracesSequence{
				Id: cell.trapId,
				Traces: []Trace{
					{
						TimeStamp: 0,
						Id:        cell.trapId, // Same ID here
						Position:  Position{X: x, Y: y},
						Symbol:    '#',
					},
				},
			}
		}
	}

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
