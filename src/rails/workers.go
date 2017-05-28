package rails

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

type WorkerSlice []*Worker
type Job struct {
	duration     int
	Workplace    *Station
	workers      WorkerSlice
	counterMutex sync.Mutex
	counter      int
}

func NewJob(t int, s *Station, ws WorkerSlice) *Job {
	return &Job{
		duration:  t,
		Workplace: s,
		workers:   ws,
		counter:   len(ws)}
}

func (j *Job) arrived(w *Worker) {
	defer j.counterMutex.Unlock()
	j.counterMutex.Lock()
	j.counter--

	if j.counter == 0 {
		for _, worker := range j.workers {
			worker.ready <- true
		}
	}
}

type Worker struct {
	id    int
	Home  *Station
	Job   *Job
	At    *Station
	In    *Train
	Done  chan bool
	ready chan bool
	Work  chan *Job
}

func NewWorker(id int, home *Station) (worker *Worker) {
	worker = &Worker{
		id:    id,
		Home:  home,
		Job:   nil,
		At:    home,
		In:    nil,
		Done:  make(chan bool),
		ready: make(chan bool),
		Work:  make(chan *Job)}
	return
}

func (w *Worker) Simulate(railway *RailwayData, data *SimulationData) {
	for {
		w.Job = <-w.Work
		logger.Printf("%s %v goes to work at %v for %dm",
			ClockTime(data), w, w.Job.Workplace, w.Job.duration)

		depT := w.Home.Trains
		arrT := w.Job.Workplace.Trains

		for _, dt := range depT {
			for _, at := range arrT {
				if dt == at {
					w.travel(at, w.Home, w.Job.Workplace)
					w.work(data)
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
							w.travel(at, sd, w.Job.Workplace)
							w.work(data)
							w.travel(at, w.At, sa)
							w.travel(dt, sa, w.Home)
							return
						}
					}
				}
			}
		}

		logger.Printf("%s %v returned from work",
			ClockTime(data), w)
	}
}

func (w *Worker) ID() int { return w.id }

type Tickets []*Ticket

type Ticket struct {
	owner       *Worker
	departure   *Station
	destination *Station
	train       *Train
}

func (w *Worker) travel(train *Train, from *Station, to *Station) {
	from.ticketsMutex.Lock()
	ticket := &Ticket{
		owner:       w,
		departure:   from,
		destination: to,
		train:       train}
	from.TicketsFor[train] = append(from.TicketsFor[train], ticket)
	from.ticketsMutex.Unlock()

	logger.Printf("%v got ticket for %v[%v->%v]",
		w, train, from, to)

	<-w.Done
}

func (w *Worker) work(data *SimulationData) {
	go w.Job.arrived(w)
	<-w.ready

	logger.Printf("%s %v is working...",
		ClockTime(data), w)

	duration := float64(w.Job.duration) / 60.0
	sleepTime := float64(data.SecondsPerHour) * duration * 1000.0
	time.Sleep(time.Duration(sleepTime) * time.Millisecond)

	logger.Printf("%s %v leaves work",
		ClockTime(data), w)
	w.Job = nil
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
