/*
 * Radoslaw Kowalski 221454
 */
package rails

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
)

const (
	TRAIN_BREAK_PROBABILITY = 0.02
)

type TrainSlice []*Train
type RepairTeamSlice []*RepairTeam

// Route is a slice of Turntable pointers that represents cycle in railroad.
type Route TurntableSlice

// Train stores all parameters for train instance needed for its simulation.
// Only exported field is Name, all operations concerning Train type are done
// by appropriate functions.
// A Train must be created using NewTrain.
type Train struct {
	id           int // Train's identification
	speed        int // maximum speed in km/h
	capacity     int // how many people can board the train
	repairTime   int
	Name         string // Train's name for pretty printing
	route        Route  // cycle on railroad represented by TurntableSlice
	index        int    // current position on route (last visited Turntable)
	at           Track  // current position, Track the train occupies
	Connects     StationSlice
	validTickets Tickets
	Seats        chan bool
	Done         chan bool
	Repaired     chan bool
	Broke        chan *Train
}

// NewTrain creates pointer to new Train type instance.
// First Turntable on route is automatically locked without checking if it is free first.
// Every Train instance should be created using NewTrain.
func NewTrain(id, speed, cap, repTime int, name string, route Route) (train *Train) {
	train = &Train{
		id:           id,
		speed:        speed,
		capacity:     cap,
		repairTime:   repTime,
		Name:         strings.Title(name),
		route:        route,
		index:        0,
		at:           route[0],
		Connects:     make(StationSlice, 0),
		validTickets: make(Tickets, 0),
		Seats:        make(chan bool, cap),
		Done:         make(chan bool),
		Repaired:     make(chan bool),
		Broke:        make(chan *Train, 1)}
	return
}

func (t *Train) Simulate(railway *RailwayData, data *SimulationData, wg *sync.WaitGroup) {
	defer wg.Done()

	logger.Printf("%s %v starts work", ClockTime(data), t)

	track := t.At().(*Turntable)
	track.Rider <- t
	<-t.Done
	<-track.Done

	for {
		select {
		case <-t.Broke:
			select {
			case railway.RepairChannel <- t:
				logger.Printf("%s %v broke", ClockTime(data), t)
				<-t.Repaired
				logger.Printf("%s %v repaired", ClockTime(data), t)
			default:
				continue
			}
		default:
			// get nearest TurntableSlice
			fst, snd := t.Connection()
		Loop1: // search for available Track connecting `fst` and `snd`
			for {
				for _, r := range railway.Connections[fst.ID()][snd.ID()] {
					switch r.(type) {
					case *StationTrack:
						r := r.(*StationTrack)
						select {
						case r.Rider <- t:
							<-r.Done
							break Loop1
						default:
							continue
						}
					case *NormalTrack:
						r := r.(*NormalTrack)
						select {
						case r.Rider <- t:
							<-r.Done
							break Loop1
						default:
							continue
						}
					}
				}
			}
			snd.Rider <- t
			t.NextPosition()
			<-snd.Done

			if rand.Float64() < TRAIN_BREAK_PROBABILITY {
				t.Broke <- t
			}
		}
	}
}

func (t *Train) letPassengersOut(station *Station, data *SimulationData) {
	left := 0
	for i := range t.validTickets {
		j := i - left
		ticket := t.validTickets[j]
		if ticket.destination == station {
			t.validTickets = append(t.validTickets[:j], t.validTickets[j+1:]...)
			left++
			<-t.Seats
			logger.Printf("%v gets off %v at %v",
				ticket.owner, t, station)
			ticket.owner.In = nil
			ticket.owner.At = station

			ticket.owner.Done <- true
		}
	}
}

func (t *Train) validateTickets(station *Station) {
	validated := 0
	for i := range station.TicketsFor[t] {
		j := i - validated
		ticket := (station.TicketsFor[t])[j]
		select {
		case t.Seats <- true:
			logger.Printf("%v gets on %v at %v",
				ticket.owner, t, station)
			station.ticketsMutex.Lock()
			station.TicketsFor[t] = append((station.TicketsFor[t])[:j], (station.TicketsFor[t])[j+1:]...)
			station.ticketsMutex.Unlock()
			validated++
			t.validTickets = append(t.validTickets, ticket)
			ticket.owner.In = t
			ticket.owner.At = nil
		default:
			return
		}
	}
}

// At returns value of tt'st un-exported field at.
func (t *Train) At() Track { return t.at }

func (t *Train) ID() int { return t.id }

func (t *Train) Speed() int { return t.speed }

// Connection returns pair of pointers to TurntableSlice in tt'st route from current at.
func (t *Train) Connection() (from, to *Turntable) {
	return t.route[t.index], t.route[(t.index+1)%len(t.route)]
}

// MoveTo unlocks tt'st old position, moving it to Track to, when it is Turntable also
// increments index of tt'st route.
// Returns stopTime tt will have to spend on new position.
// MoveTo should be used after after successful lock on next position.
func (t *Train) SetAt(at Track) { t.at = at }

func (t *Train) NextPosition() { t.index = (t.index + 1) % len(t.route) }

// String returns human-friendly label for Train t
func (t *Train) String() string { return fmt.Sprintf("Train%d %s", t.id, strings.ToUpper(t.Name)) }

// GoString returns more verbose human-friendly representation of Train t
func (t *Train) GoString() string {
	return fmt.Sprintf(
		"rails.Train:%s:%d{speed:%d, cap:%d, RepairTime:%d, route:%s, at:%s, Connects:%s}",
		t.Name, t.id, t.speed, t.capacity, t.repairTime, t.route, t.at, t.Connects)
}

func (r Route) String() string {
	ids := make([]string, len(r))
	for i, tt := range r {
		ids[i] = strconv.Itoa(tt.id)
	}
	return "[" + strings.Join(ids, " ") + "]"
}

// GoString prints all Route's content as GoString
func (r Route) GoString() string {
	ids := make([]string, len(r))
	for i, tt := range r {
		ids[i] = tt.GoString()
	}
	return "rails.Route{" + strings.Join(ids, ", ") + "}"
}
