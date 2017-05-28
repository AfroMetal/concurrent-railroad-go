/*
 * Radoslaw Kowalski 221454
 */

// Package rails implements types and operations for railroad simulation.
package rails

import (
	"fmt"
	"math"
	"math/rand"
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

type TurntableSlice []*Turntable
type NormalTrackSlice []*NormalTrack
type StationTrackSlice []*StationTrack

// Track is an interface for NormalTrack, StationTrack, Turntable that enables
// basic operations on them without knowing precise type
type Track interface {
	Sleep(speed int, sph int)
	ID() int
	Reserve() bool
	Cancel()
	isAvailable() bool
	Neighbors(connections ConnectionsGraph) (ns Neighbors)
	Simulate(railway *RailwayData, data *SimulationData)
	String() string
	GoString() string
}

// NormalTrack represents Track interface implementation that is used to pass distance between two TurntableSlice.
// It is an edge in railroad representation.
type NormalTrack struct {
	id         int // identification
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

// StationTrack represents Track interface implementation to stationed TrainSlice.
// It is an edge in railroad representation.
type StationTrack struct {
	id         int // identification
	stopTime   int // minimum stopTime on station in minutes
	repairTime int
	Name       string
	first      *Turntable
	second     *Turntable
	station    *Station
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
	id         int // identification
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

func (nt *NormalTrack) Simulate(railway *RailwayData, data *SimulationData) {
	for {
		select {
		case <-nt.Broke:
			select {
			case railway.RepairChannel <- nt:
				logger.Printf("%s %v broke", ClockTime(data), nt)
				<-nt.Repaired
				logger.Printf("%s %v repaired", ClockTime(data), nt)
			default:
				continue
			}
		case <-nt.Reserved:
			select {
			case <-nt.Cancelled:
				continue
			case rt := <-nt.TeamRider:
				rt.Done <- true

				rt.SetAt(nt)
				logger.Printf("%s %v travels along reserved %v",
					ClockTime(data), rt, nt)
				nt.Sleep(rt.Speed(), data.SecondsPerHour)

				nt.Done <- true
				<-rt.Done
			}
		case t := <-nt.Rider:
			t.Done <- true

			t.SetAt(nt)
			logger.Printf("%s %v travels along %v",
				ClockTime(data), t, nt)
			nt.Sleep(t.Speed(), data.SecondsPerHour)

			nt.Done <- true
			<-t.Done
			if rand.Float64() < 0.05 {
				nt.Broke <- nt
			}
		case rt := <-nt.TeamRider:
			rt.Done <- true

			rt.SetAt(nt)
			logger.Printf("%s %v travels along %v",
				ClockTime(data), rt, nt)
			nt.Sleep(rt.Speed(), data.SecondsPerHour)

			nt.Done <- true
			<-rt.Done
		}
	}
}

func (st *StationTrack) Simulate(railway *RailwayData, data *SimulationData) {
	for {
		select {
		case <-st.Broke:
			select {
			case railway.RepairChannel <- st:
				logger.Printf("%s %v broke", ClockTime(data), st)
				<-st.Repaired
				logger.Printf("%s %v repaired", ClockTime(data), st)
			default:
				continue
			}
		case <-st.Reserved:
			select {
			case <-st.Cancelled:
				continue
			case rt := <-st.TeamRider:
				rt.Done <- true

				rt.SetAt(st)
				logger.Printf("%s %v waits on reserved %v",
					ClockTime(data), rt, st)
				st.Sleep(rt.Speed(), data.SecondsPerHour)

				st.Done <- true
				<-rt.Done
			}
		case t := <-st.Rider:
			t.Done <- true

			*data.StatisticsChannel <- fmt.Sprintf("%v %s >-\t%v\n",
				t, ClockTime(data), st)
			// calculate real seconds to simulate action time
			t.SetAt(st)
			logger.Printf("%s %v waits on %v",
				ClockTime(data), t, st)

			t.letPassengersOut(st.station, data)
			t.validateTickets(st.station)

			st.Sleep(t.Speed(), data.SecondsPerHour)

			st.Done <- true
			<-t.Done
			if rand.Float64() < 0.01 {
				st.Broke <- st
			}
		case rt := <-st.TeamRider:
			rt.Done <- true

			rt.SetAt(st)
			logger.Printf("%s %v waits on %v",
				ClockTime(data), rt, st)
			st.Sleep(rt.Speed(), data.SecondsPerHour)

			st.Done <- true
			<-rt.Done
		}
	}
}

func (tt *Turntable) Simulate(railway *RailwayData, data *SimulationData) {
	for {
		select {
		case <-tt.Broke:
			select {
			case railway.RepairChannel <- tt:
				logger.Printf("%s %v broke", ClockTime(data), tt)
				<-tt.Repaired
				logger.Printf("%s %v repaired", ClockTime(data), tt)
			default:
				continue
			}
		case <-tt.Reserved:
			select {
			case <-tt.Cancelled:
				continue
			case rt := <-tt.TeamRider:
				rt.Done <- true

				rt.SetAt(tt)
				logger.Printf("%s %v rotates at reserved %v",
					ClockTime(data), rt, tt)
				tt.Sleep(rt.Speed(), data.SecondsPerHour)
				tt.Done <- true
				<-rt.Done
			}
		case t := <-tt.Rider:
			t.Done <- true

			switch t.At().(type) {
			// if train left station save it to timetable
			case *StationTrack:
				*data.StatisticsChannel <- fmt.Sprintf("%v %s ->\t%v\n",
					t, ClockTime(data), t.At())
			}
			// calculate real seconds to simulate action time
			t.SetAt(tt)
			logger.Printf("%s %v rotates at %v",
				ClockTime(data), t, tt)
			tt.Sleep(t.Speed(), data.SecondsPerHour)

			tt.Done <- true
			<-t.Done
			if rand.Float64() < 0.08 {
				tt.Broke <- tt
			}
		case rt := <-tt.TeamRider:
			rt.Done <- true

			rt.SetAt(tt)
			logger.Printf("%s %v rotates at %v",
				ClockTime(data), rt, tt)
			tt.Sleep(rt.Speed(), data.SecondsPerHour)

			tt.Done <- true
			<-rt.Done
		}
	}
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

func (nt *NormalTrack) Cancel()  { nt.Cancelled <- true }
func (st *StationTrack) Cancel() { st.Cancelled <- true }
func (tt *Turntable) Cancel()    { tt.Cancelled <- true }

func (nt *NormalTrack) isAvailable() bool {
	select {
	case <-nt.Available:
		return true
	default:
		return false
	}
}
func (st *StationTrack) isAvailable() bool {
	select {
	case <-st.Available:
		return true
	default:
		return false
	}
}
func (tt *Turntable) isAvailable() bool {
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

func (st *StationTrack) BelongsTo(s *Station) bool {
	return st.first == s.first && st.second == s.second
}
func (st *StationTrack) SetStation(s *Station) {
	st.station = s
}
func (st *StationTrack) Station() *Station { return st.station }
