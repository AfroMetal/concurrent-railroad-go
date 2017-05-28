package rails

import (
	"fmt"
	"math/rand"
	"time"
)

type WorkerSlice []*Worker

type Worker struct {
	id        int
	Home      *Station
	Workplace *Station
	workTime  int
	At        *Station
	In        *Train
	Done      chan bool
	ready     chan bool
}

func NewWorker(id int, home *Station) (worker *Worker) {
	worker = &Worker{
		id:        id,
		Home:      home,
		Workplace: nil,
		workTime:  0,
		At:        home,
		In:        nil,
		Done:      make(chan bool),
		ready:     make(chan bool)}
	return
}

func (w *Worker) ID() int { return w.id }

func (w *Worker) GoToWork(place *Station, workTime, sph int) {
	w.Workplace = place
	w.workTime = workTime

	depT := w.Home.Trains
	arrT := w.Workplace.Trains

	for _, dt := range depT {
		for _, at := range arrT {
			if dt == at {
				w.travel(at, w.Home, w.Workplace)
				w.work(sph)
				w.travel(at, w.At, w.Home)
				return
			}
		}
	}

	// no direct connection, look for change

	for _, dt := range depT {
		for _, sd := range dt.Connects {
			for _, at := range arrT {
				for _, sa := range at.Connects {
					if sd == sa {
						w.travel(dt, w.Home, sd)
						w.travel(at, sd, w.Workplace)
						w.work(sph)
						w.travel(at, w.At, sa)
						w.travel(dt, sa, w.Home)
						return
					}
				}
			}
		}
	}
}

type Tickets []*Ticket

type Ticket struct {
	owner       *Worker
	departure   *Station
	destination *Station
	train       *Train
}

func (w *Worker) travel(train *Train, from *Station, to *Station) {
	// TODO: travel...
	from.ticketsMutex.Lock()
	from.TicketsFor[train] = append(from.TicketsFor[train], &Ticket{
		owner:       w,
		departure:   from,
		destination: to,
		train:       train})
	from.ticketsMutex.Unlock()
	<-w.Done
}

func (w *Worker) work(sph int) {
	// TODO: wait for all
	//teammates := make(WorkerSlice, 0)
	//for _, w := range
	//
	//<-w.ready

	logger.Printf("%v is working...", w)
	duration := float64(w.workTime) / 60.0
	sleepTime := float64(sph) * duration * 1000.0
	time.Sleep(time.Duration(sleepTime) * time.Millisecond)
	w.workTime = 0
	w.Workplace = nil
}

func (w *Worker) String() string {
	return fmt.Sprintf("Worker%d from %s", w.id, w.Home.Name)
}

func (ws WorkerSlice) Subset(n int) (subset WorkerSlice) {
	if n > len(ws) {
		panic("Can't generate random subset larger than set")
	}
	rand.Seed(time.Now().UnixNano())
	for _, i := range rand.Perm(len(ws))[:n] {
		subset = append(subset, ws[i])
	}
	return subset
}
