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
}

// NewTrain creates pointer to new Train type instance.
// First Turntable on route is automatically locked without checking if it is free first.
// Every Train instance should be created using NewTrain.
func NewTrain(id, speed, cap int, name string, route Route) (train *Train) {
	train = &Train{id, speed, cap, strings.Title(name), route, 0, route[0]}
	train.at.Lock()
	return
}

// At returns value of tt'st un-exported field at.
func (t *Train) At() Track { return t.at }

// Connection returns pair of pointers to Turntables in tt'st route from current at.
func (t *Train) Connection() (from, to *Turntable) {
	return t.route[t.index], t.route[(t.index+1)%len(t.route)]
}

// MoveTo unlocks tt'st old position, moving it to Track to, when it is Turntable also
// increments index of tt'st route.
// Returns time tt will have to spend on new position.
// MoveTo should be used after after successful lock on next position.
func (t *Train) MoveTo(to Track) (time float64) {
	t.at.Unlock()
	t.at = to
	switch to.(type) {
	case *Turntable:
		t.index = (t.index + 1) % len(t.route)
	}
	return t.at.actionTime(t)
}

// Delay pauses Train tt for at least st seconds.
func (t *Train) Delay(s float64) { time.Sleep(time.Duration(s) * time.Second) }

// String returns human-friendly label for Train t
func (t *Train) String() string { return fmt.Sprintf("Train%d %s", t.id, t.Name) }

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
	actionTime(train *Train) float64 // returns time needed before track can be left
	ID() int
	GetLock() bool
	Lock()
	Unlock()
	String() string
	GoString() string
}

// NormalTrack represents Track interface implementation that is used to pass distance between two Turntables.
// It is an edge in railroad representation.
type NormalTrack struct {
	id    int // identificator
	len   int // track length in km
	limit int // speed limit on trac in km/h
	lock  chan bool
}

// StationTrack represents Track interface implementation to stationed Trains.
// It is an edge in railroad representation.
type StationTrack struct {
	id   int // identificator
	time int // minimum time on station in minutes
	Name string
	lock chan bool
}

// Turntable represents Track interface implementation to rotate Train and move from one track to another.
// Turntable is a node connecting edges in railroad representation.
type Turntable struct {
	id   int // identificator
	time int // minimum time needed to rotate the train
	lock chan bool
}

// Route is a slice of Turntable pointers that represents cycle in railroad.
type Route []*Turntable

// NewNormalTrack creates pointer to new NormalTrack type instance.
// Created NormalTrack is unlocked.
// NormalTrack should always be created using NewNormalTrack.
func NewNormalTrack(id, len, limit int) (nt *NormalTrack) {
	nt = &NormalTrack{id, len, limit, make(chan bool, 1)}
	nt.Unlock()
	return
}

// NewStationTrack creates pointer to new StationTrack type instance.
// Created StationTrack is unlocked.
// StationTrack should always be created using NewStationTrack.
func NewStationTrack(id int, name string, time int) (st *StationTrack) {
	st = &StationTrack{id, time, strings.ToUpper(name), make(chan bool, 1)}
	st.Unlock()
	return
}

// NewTurntable creates pointer to new Turntable type instance.
// Created Turntable is unlocked.
// Turntable should always be created using NewTurntable.
func NewTurntable(id, time int) (tt *Turntable) {
	tt = &Turntable{id, time, make(chan bool, 1)}
	tt.Unlock()
	return
}

// actionTime returns time in simulation hours that traveling along NormalTrack will take.
func (nt *NormalTrack) actionTime(train *Train) (time float64) {
	return float64(nt.len) / math.Min(float64(nt.limit), float64(train.speed))
}

// actionTime returns time in simulation hours that stationing on StationTrack will take.
func (st *StationTrack) actionTime(train *Train) (time float64) { return float64(st.time) / 60.0 }

// actionTime returns time in simulation hours that rotating on Turntable will take.
func (tt *Turntable) actionTime(train *Train) (time float64) { return float64(tt.time) / 60.0 }

// ID returns unexported field id
func (nt *NormalTrack) ID() int { return nt.id }

// ID returns unexported field id
func (st *StationTrack) ID() int { return st.id }

// ID returns unexported field id
func (tt *Turntable) ID() int { return tt.id }

// GetLock tries to acquire lock on synchronization channel, return true if it succeeded, false otherwise.
func (nt *NormalTrack) GetLock() (success bool) {
	select {
	case <-nt.lock:
		return true
	default:
		return false
	}
}

// GetLock tries to acquire lock on synchronization channel, return true if it succeeded, false otherwise.
func (st *StationTrack) GetLock() (success bool) {
	select {
	case <-st.lock:
		return true
	default:
		return false
	}
}

// GetLock tries to acquire lock on synchronization channel, return true if it succeeded, false otherwise.
func (tt *Turntable) GetLock() (success bool) {
	select {
	case <-tt.lock:
		return true
	default:
		return false
	}
}

// Lock acquires lock on synchronization channel, it will block calling goroutine until it is possible.
func (nt *NormalTrack) Lock() { <-nt.lock }

// Lock acquires lock on synchronization channel, it will block calling goroutine until it is possible.
func (st *StationTrack) Lock() { <-st.lock }

// Lock acquires lock on synchronization channel, it will block calling goroutine until it is possible.
func (tt *Turntable) Lock() { <-tt.lock }

// Unlock releases lock on synchronization channel.
func (nt *NormalTrack) Unlock() { nt.lock <- true }

// Unlock releases lock on synchronization channel.
func (st *StationTrack) Unlock() { st.lock <- true }

// Unlock releases lock on synchronization channel.
func (tt *Turntable) Unlock() { tt.lock <- true }

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
