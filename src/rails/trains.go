package rails

import (
	"fmt"
	"strconv"
	"strings"
)

type Trains []*Train
type RepairTeams []*RepairTeam

// Route is a slice of Turntable pointers that represents cycle in railroad.
type Route Turntables

// Train stores all parameters for train instance needed for its simulation.
// Only exported field is Name, all operations concerning Train type are done
// by appropriate functions.
// A Train must be created using NewTrain.
type Train struct {
	id         int // Train's identification
	speed      int // maximum speed in km/h
	capacity   int // how many people can board the train
	repairTime int
	Name       string // Train's name for pretty printing
	route      Route  // cycle on railroad represented by Turntables
	index      int    // current position on route (last visited Turntable)
	at         Track  // current position, Track the train occupies
	connects   Stations
	Seats      chan bool
	Done       chan bool
	Repaired   chan bool
	Broke      chan *Train
}

// NewTrain creates pointer to new Train type instance.
// First Turntable on route is automatically locked without checking if it is free first.
// Every Train instance should be created using NewTrain.
func NewTrain(id, speed, cap, repTime int, name string, route Route, connections Stations) (train *Train) {
	train = &Train{
		id:         id,
		speed:      speed,
		capacity:   cap,
		repairTime: repTime,
		Name:       strings.Title(name),
		route:      route,
		index:      0,
		at:         route[0],
		connects:   connections,
		Seats:      make(chan bool, cap),
		Done:       make(chan bool),
		Repaired:   make(chan bool),
		Broke:      make(chan *Train, 1)}
	return
}

// At returns value of tt'st un-exported field at.
func (t *Train) At() Track { return t.at }

func (t *Train) ID() int { return t.id }

func (t *Train) Speed() int { return t.speed }

// Connection returns pair of pointers to Turntables in tt'st route from current at.
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
		"rails.Train:%s:%d{speed:%d, cap:%d, RepairTime:%d, route:%s, at:%s, connects:%s}",
		t.Name, t.id, t.speed, t.capacity, t.repairTime, t.route, t.at, t.connects)
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
