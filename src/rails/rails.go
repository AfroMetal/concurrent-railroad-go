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

type ConnectionsGraph []map[int][]Track

func NewConnectionsGraph(n int) (connections ConnectionsGraph) {
	connections = make(ConnectionsGraph, n)
	for i := range connections {
		connections[i] = make(map[int][]Track)
		for j := range connections[i] {
			connections[i][j] = []Track{}
		}
	}
	return
}

type Turntables []*Turntable
type NormalTracks []*NormalTrack
type StationTracks []*StationTrack
type Trains []*Train
type RepairTeams []*RepairTeam

// Train stores all parameters for train instance needed for its simulation.
// Only exported field is Name, all operations concerning Train type are done
// by appropriate functions.
// A Train must be created using NewTrain.
type Train struct {
	id         int // Train's identificator
	speed      int // maximum speed in km/h
	capacity   int // how many people can board the train
	repairTime int
	Name       string // Train's name for pretty printing
	route      Route  // cycle on railroad represented by Turntables
	index      int    // current position on route (last visited Turntable)
	at         Track  // current position, Track the train occupies
	Done       chan bool
	Repaired   chan bool
	Broke      chan *Train
}

// NewTrain creates pointer to new Train type instance.
// First Turntable on route is automatically locked without checking if it is free first.
// Every Train instance should be created using NewTrain.
func NewTrain(id, speed, cap, repTime int, name string, route Route) (train *Train) {
	train = &Train{
		id:         id,
		speed:      speed,
		capacity:   cap,
		repairTime: repTime,
		Name:       strings.Title(name),
		route:      route,
		index:      0,
		at:         route[0],
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
func (t *Train) SetAt(at Track) {
	t.at = at
}

func (t *Train) NextPosition() {
	t.index = (t.index + 1) % len(t.route)
}

// String returns human-friendly label for Train t
func (t *Train) String() string { return fmt.Sprintf("Train%d %s", t.id, strings.ToUpper(t.Name)) }

// GoString returns more verbose human-friendly representation of Train t
func (t *Train) GoString() string {
	return fmt.Sprintf(
		"rails.Train:%s:%d{speed:%d, cap:%d, RepairTime:%d, route:%s, at:%s}",
		t.Name, t.id, t.speed, t.capacity, t.repairTime, t.route, t.at)
}

// Route is a slice of Turntable pointers that represents cycle in railroad.
type Route Turntables

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

type Neighbors []Track
type Path []Track

type BrokenFella interface {
	RepairTime() float64
	Repair()
	Neighbors(connections ConnectionsGraph) (ns Neighbors)
}

func (t *Train) RepairTime() float64 { return float64(t.repairTime) / 60.0 }

func (tt *Turntable) RepairTime() float64 { return float64(tt.repairTime) / 60.0 }

func (nt *NormalTrack) RepairTime() float64 { return float64(nt.repairTime) / 60.0 }

func (st *StationTrack) RepairTime() float64 { return float64(st.repairTime) / 60.0 }

func (t *Train) Repair() { t.Repaired <- true }

func (tt *Turntable) Repair() { tt.Repaired <- true }

func (nt *NormalTrack) Repair() { nt.Repaired <- true }

func (st *StationTrack) Repair() { st.Repaired <- true }

func (t *Train) Neighbors(connections ConnectionsGraph) (ns Neighbors) {
	pos := t.at
	return pos.(BrokenFella).Neighbors(connections)
}

func (tt *Turntable) Neighbors(connections ConnectionsGraph) (ns Neighbors) {
	i := tt.id
	for j := range connections[i] {
		for _, track := range connections[i][j] {
			ns = append(ns, track)
		}
	}
	return
}

func (nt *NormalTrack) Neighbors(connections ConnectionsGraph) (ns Neighbors) {
	ns = Neighbors{nt.first, nt.second}
	return
}

func (st *StationTrack) Neighbors(connections ConnectionsGraph) (ns Neighbors) {
	ns = Neighbors{st.first, st.second}
	return
}

type RepairTeam struct {
	id      int // Train's identificator
	speed   int // maximum speed in km/h
	station *StationTrack
	Done    chan bool
}

func NewRepairTeam(id, speed int, station *StationTrack) (team *RepairTeam) {
	team = &RepairTeam{
		id:      id,
		speed:   speed,
		station: station,
		Done:    make(chan bool)}
	return
}

func (rt *RepairTeam) Station() *StationTrack { return rt.station }
func (rt *RepairTeam) Speed() int             { return rt.speed }

// String returns human-friendly label for Train t
func (rt *RepairTeam) String() string { return fmt.Sprintf("RepairTeam%d", rt.id) }

// GoString returns more verbose human-friendly representation of Train t
func (rt *RepairTeam) GoString() string {
	return fmt.Sprintf(
		"rails.RepairTeam:%d{speed:%d, stationId:%d}",
		rt.id, rt.speed, rt.station.id)
}

func SearchForPath(currentPath Path, from Track, destination Neighbors, resp chan Path, graph ConnectionsGraph) {
	for _, track := range from.Neighbors(graph) {
		switch track.IsAvailable() {
		case true:
			for _, d := range destination {
				if track == d {
					select {
					case resp <- append(currentPath, track):
						return
					default:
						continue
					}
				}
			}
			SearchForPath(append(currentPath, track), track, destination, resp, graph)
		default:
			continue
		}
	}
}

// Track is an interface for NormalTrack, StationTrack, Turntable that enables
// basic operations on them without knowing precise type
type Track interface {
	Sleep(speed int, sph int)
	ID() int
	Reserve() bool
	Do()
	Cancel()
	IsAvailable() bool
	Neighbors(connections ConnectionsGraph) (ns Neighbors)
	String() string
	GoString() string
}

// NormalTrack represents Track interface implementation that is used to pass distance between two Turntables.
// It is an edge in railroad representation.
type NormalTrack struct {
	id         int // identificator
	len        int // track length in km
	limit      int // speed limit on track in km/h
	repairTime int
	first      *Turntable
	second     *Turntable
	Rider      chan *Train
	TeamRider  chan *RepairTeam
	Done       chan bool
	Reserved   chan bool
	Available  chan bool
	Cancelled  chan bool
	Repaired   chan bool
	Broke      chan *NormalTrack
}

// StationTrack represents Track interface implementation to stationed Trains.
// It is an edge in railroad representation.
type StationTrack struct {
	id         int // identificator
	stopTime   int // minimum stopTime on station in minutes
	repairTime int
	Name       string
	first      *Turntable
	second     *Turntable
	Rider      chan *Train
	TeamRider  chan *RepairTeam
	Done       chan bool
	Reserved   chan bool
	Available  chan bool
	Cancelled  chan bool
	Repaired   chan bool
	Broke      chan *StationTrack
}

// Turntable represents Track interface implementation to rotate Train and move from one track to another.
// Turntable is a node connecting edges in railroad representation.
type Turntable struct {
	id         int // identificator
	turnTime   int // minimum stopTime needed to rotate the train
	repairTime int
	Rider      chan *Train
	TeamRider  chan *RepairTeam
	Done       chan bool
	Reserved   chan bool
	Available  chan bool
	Cancelled  chan bool
	Repaired   chan bool
	Broke      chan *Turntable
}

// NewNormalTrack creates pointer to new NormalTrack type instance.
// Created NormalTrack is unlocked.
// NormalTrack should always be created using NewNormalTrack.
func NewNormalTrack(id, len, limit, repTime int, fst, snd *Turntable) (nt *NormalTrack) {
	nt = &NormalTrack{
		id:         id,
		len:        len,
		limit:      limit,
		repairTime: repTime,
		first:      fst,
		second:     snd,
		Rider:      make(chan *Train),
		TeamRider:  make(chan *RepairTeam),
		Done:       make(chan bool),
		Reserved:   make(chan bool),
		Available:  make(chan bool, 1),
		Cancelled:  make(chan bool),
		Repaired:   make(chan bool),
		Broke:      make(chan *NormalTrack, 1)}
	return
}

// NewStationTrack creates pointer to new StationTrack type instance.
// Created StationTrack is unlocked.
// StationTrack should always be created using NewStationTrack.
func NewStationTrack(id int, name string, time, repTime int, fst, snd *Turntable) (st *StationTrack) {
	st = &StationTrack{
		id:         id,
		stopTime:   time,
		repairTime: repTime,
		Name:       strings.ToUpper(name),
		first:      fst,
		second:     snd,
		Rider:      make(chan *Train),
		TeamRider:  make(chan *RepairTeam),
		Done:       make(chan bool),
		Reserved:   make(chan bool),
		Available:  make(chan bool, 1),
		Cancelled:  make(chan bool),
		Repaired:   make(chan bool),
		Broke:      make(chan *StationTrack, 1)}
	return
}

// NewTurntable creates pointer to new Turntable type instance.
// Created Turntable is unlocked.
// Turntable should always be created using NewTurntable.
func NewTurntable(id, time, repTime int) (tt *Turntable) {
	tt = &Turntable{
		id:         id,
		turnTime:   time,
		repairTime: repTime,
		Rider:      make(chan *Train),
		TeamRider:  make(chan *RepairTeam),
		Done:       make(chan bool),
		Reserved:   make(chan bool),
		Available:  make(chan bool, 1),
		Cancelled:  make(chan bool),
		Repaired:   make(chan bool),
		Broke:      make(chan *Turntable, 1)}
	return
}

// ActionTime returns stopTime in simulation hours that traveling along NormalTrack will take.
func (nt *NormalTrack) Sleep(speed int, sph int) {
	duration := float64(nt.len) / math.Min(float64(nt.limit), float64(speed))
	sleepTime := float64(sph) * duration * 1000.0
	time.Sleep(time.Duration(sleepTime) * time.Millisecond)
}

// ActionTime returns stopTime in simulation hours that stationing on StationTrack will take.
func (st *StationTrack) Sleep(speed int, sph int) {
	duration := float64(st.stopTime) / 60.0
	sleepTime := float64(sph) * duration * 1000.0
	time.Sleep(time.Duration(sleepTime) * time.Millisecond)
}

// ActionTime returns stopTime in simulation hours that rotating on Turntable will take.
func (tt *Turntable) Sleep(speed int, sph int) {
	duration := float64(tt.turnTime) / 60.0
	sleepTime := float64(sph) * duration * 1000.0
	time.Sleep(time.Duration(sleepTime) * time.Millisecond)
}

// ID returns unexported field id
func (nt *NormalTrack) ID() int { return nt.id }

// ID returns unexported field id
func (st *StationTrack) ID() int { return st.id }

// ID returns unexported field id
func (tt *Turntable) ID() int { return tt.id }

func (nt *NormalTrack) Reserve() bool {
	select {
	case nt.Reserved <- true:
		select {
		case nt.Available <- true:
		default:
			<-nt.Available
			nt.Available <- true
		}
		return true
	default:
		return false
	}
}
func (st *StationTrack) Reserve() bool {
	select {
	case st.Reserved <- true:
		select {
		case st.Available <- true:
		default:
			<-st.Available
			st.Available <- true
		}
		return true
	default:
		return false
	}
}
func (tt *Turntable) Reserve() bool {
	select {
	case tt.Reserved <- true:
		select {
		case tt.Available <- true:
		default:
			<-tt.Available
			tt.Available <- true
		}
		return true
	default:
		return false
	}
}

func (nt *NormalTrack) Do() {
	nt.Done <- true
}
func (st *StationTrack) Do() {
	st.Done <- true
}
func (tt *Turntable) Do() {
	tt.Done <- true
}

func (nt *NormalTrack) Cancel() {
	nt.Cancelled <- true
}
func (st *StationTrack) Cancel() {
	st.Cancelled <- true
}
func (tt *Turntable) Cancel() {
	tt.Cancelled <- true
}

func (nt *NormalTrack) IsAvailable() bool {
	select {
	case <-nt.Available:
		return true
	default:
		return false
	}
}
func (st *StationTrack) IsAvailable() bool {
	select {
	case <-st.Available:
		return true
	default:
		return false
	}
}
func (tt *Turntable) IsAvailable() bool {
	select {
	case <-tt.Available:
		return true
	default:
		return false
	}
}

func (nt *NormalTrack) Siblings(connections ConnectionsGraph) []Track {
	return connections[nt.first.id][nt.second.id]
}
func (st *StationTrack) Siblings(connections ConnectionsGraph) []Track {
	return connections[st.first.id][st.second.id]
}

// String returns human-friendly label for NormalTrack
func (nt *NormalTrack) String() string { return "NormalTrack" + strconv.Itoa(nt.id) }

// String returns human-friendly label for StationTrack
func (st *StationTrack) String() string { return fmt.Sprintf("StationTrack%d %s", st.id, st.Name) }

// String returns human-friendly label for Turntable
func (tt *Turntable) String() string { return "Turntable" + strconv.Itoa(tt.id) }

// GoString returns more verbose human-friendly representation for NormalTrack
func (nt *NormalTrack) GoString() string {
	return fmt.Sprintf(
		"rails.NormalTrack:%d{len:%d, limit:%d, RepairTime:%d}",
		nt.id, nt.len, nt.limit, nt.repairTime)
}

// GoString returns more verbose human-friendly representation for StationTrack
func (st *StationTrack) GoString() string {
	return fmt.Sprintf(
		"rails.StationTrack:%d:%s{stopTime:%d, RepairTime:%d}",
		st.id, st.Name, st.stopTime, st.repairTime)
}

// GoString returns more verbose human-friendly representation for Turntable
func (tt *Turntable) GoString() string {
	return fmt.Sprintf(
		"rails.Turntable:%d{stopTime:%d, RepairTime:%d}",
		tt.id, tt.turnTime, tt.repairTime)
}
