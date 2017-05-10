/*
 * Radoslaw Kowalski 221454
 */

// Package rails implements types and operations for railroad simulation.
package rails

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Train stores all parameters for train instance needed for its simulation.
// Only exported field is Name, all operations concerning Train type are done
// by appropriate functions.
// A Train must be created using NewTrain.
type Train struct {
	id       int    // Train's identificator
	speed    int    // maximum speed in km/h
	capacity int    // how many people can board the train
	Name     string // Train's name for pretty printing
	route    Route  // cycle on railroad represented by Turntables
	index    int    // current position on route (last visited Turntable)
	at       Track  // current position, Track the train occupies
	Done     chan bool
}

// NewTrain creates pointer to new Train type instance.
// First Turntable on route is automatically locked without checking if it is free first.
// Every Train instance should be created using NewTrain.
func NewTrain(id, speed, cap int, name string, route Route) (train *Train) {
	train = &Train{id, speed, cap, strings.Title(name), route, 0, route[0], make(chan bool)}
	return
}

// At returns value of tt'st un-exported field at.
func (t *Train) At() Track { return t.at }

func (t *Train) ID() int { return t.id }

// Connection returns pair of pointers to Turntables in tt'st route from current at.
func (t *Train) Connection() (from, to *Turntable) {
	return t.route[t.index], t.route[(t.index+1)%len(t.route)]
}

// MoveTo unlocks tt'st old position, moving it to Track to, when it is Turntable also
// increments index of tt'st route.
// Returns time tt will have to spend on new position.
// MoveTo should be used after after successful lock on next position.
func (t *Train) SetAt(at Track) {
	t.at = at
}

func (t *Train) NextPosition() {
	t.index = (t.index + 1) % len(t.route)
}

// Delay pauses Train tt for at least st seconds.
func (t *Train) Delay(s float64) { time.Sleep(time.Duration(s) * time.Second) }

// String returns human-friendly label for Train t
func (t *Train) String() string { return fmt.Sprintf("Train%d %s", t.id, strings.ToUpper(t.Name)) }

// GoString returns more verbose human-friendly representation of Train t
func (t *Train) GoString() string {
	return fmt.Sprintf(
		"rails.Train:%s:%d{speed:%d, cap:%d, route:%s, at:%s}",
		t.Name, t.id, t.speed, t.capacity, t.route, t.at)
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

// Track is an interface for NormalTrack, StationTrack, Turntable that enables
// basic operations on them without knowing precise type
type Track interface {
	Sleep(train *Train, sph int)
	ID() int
	String() string
	GoString() string
}

// NormalTrack represents Track interface implementation that is used to pass distance between two Turntables.
// It is an edge in railroad representation.
type NormalTrack struct {
	id    int // identificator
	len   int // track length in km
	limit int // speed limit on trac in km/h
	Rider chan *Train
	Done  chan bool
}

// StationTrack represents Track interface implementation to stationed Trains.
// It is an edge in railroad representation.
type StationTrack struct {
	id    int // identificator
	time  int // minimum time on station in minutes
	Name  string
	Rider chan *Train
	Done  chan bool
}

// Turntable represents Track interface implementation to rotate Train and move from one track to another.
// Turntable is a node connecting edges in railroad representation.
type Turntable struct {
	id    int // identificator
	time  int // minimum time needed to rotate the train
	Rider chan *Train
	Done  chan bool
}

// Route is a slice of Turntable pointers that represents cycle in railroad.
type Route []*Turntable

// NewNormalTrack creates pointer to new NormalTrack type instance.
// Created NormalTrack is unlocked.
// NormalTrack should always be created using NewNormalTrack.
func NewNormalTrack(id, len, limit int) (nt *NormalTrack) {
	nt = &NormalTrack{
		id:    id,
		len:   len,
		limit: limit,
		Rider: make(chan *Train),
		Done:  make(chan bool)}
	return
}

// NewStationTrack creates pointer to new StationTrack type instance.
// Created StationTrack is unlocked.
// StationTrack should always be created using NewStationTrack.
func NewStationTrack(id int, name string, time int) (st *StationTrack) {
	st = &StationTrack{
		id:    id,
		time:  time,
		Name:  strings.ToUpper(name),
		Rider: make(chan *Train),
		Done:  make(chan bool)}
	return
}

// NewTurntable creates pointer to new Turntable type instance.
// Created Turntable is unlocked.
// Turntable should always be created using NewTurntable.
func NewTurntable(id, time int) (tt *Turntable) {
	tt = &Turntable{
		id:    id,
		time:  time,
		Rider: make(chan *Train),
		Done:  make(chan bool)}
	return
}

// ActionTime returns time in simulation hours that traveling along NormalTrack will take.
func (nt *NormalTrack) Sleep(train *Train, sph int) {
	duration := float64(nt.len) / math.Min(float64(nt.limit), float64(train.speed))
	sleepTime := float64(sph) * duration
	time.Sleep(time.Duration(sleepTime) * time.Second)
}

// ActionTime returns time in simulation hours that stationing on StationTrack will take.
func (st *StationTrack) Sleep(train *Train, sph int) {
	duration := float64(st.time) / 60.0
	sleepTime := float64(sph) * duration
	time.Sleep(time.Duration(sleepTime) * time.Second)
}

// ActionTime returns time in simulation hours that rotating on Turntable will take.
func (tt *Turntable) Sleep(train *Train, sph int) {
	duration := float64(tt.time) / 60.0
	sleepTime := float64(sph) * duration
	time.Sleep(time.Duration(sleepTime) * time.Second)
}

// ID returns unexported field id
func (nt *NormalTrack) ID() int { return nt.id }

// ID returns unexported field id
func (st *StationTrack) ID() int { return st.id }

// ID returns unexported field id
func (tt *Turntable) ID() int { return tt.id }

// String returns human-friendly label for NormalTrack
func (nt *NormalTrack) String() string { return "NormalTrack" + strconv.Itoa(nt.id) }

// String returns human-friendly label for StationTrack
func (st *StationTrack) String() string { return fmt.Sprintf("StationTrack%d %s", st.id, st.Name) }

// String returns human-friendly label for Turntable
func (tt *Turntable) String() string { return "Turntable " + strconv.Itoa(tt.id) }

// GoString returns more verbose human-friendly representation for NormalTrack
func (nt *NormalTrack) GoString() string {
	return fmt.Sprintf(
		"rails.NormalTrack:%d{len:%d, limit:%d}",
		nt.id, nt.len, nt.limit)
}

// GoString returns more verbose human-friendly representation for StationTrack
func (st *StationTrack) GoString() string {
	return fmt.Sprintf(
		"rails.StationTrack:%d:%s{time:%d}",
		st.id, st.Name, st.time)
}

// GoString returns more verbose human-friendly representation for Turntable
func (tt *Turntable) GoString() string {
	return fmt.Sprintf(
		"rails.Turntable:%d{time:%d}",
		tt.id, tt.time)
}
