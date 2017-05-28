package rails

import (
	"fmt"
	"time"
)

type Neighbors []Track
type Path []Track

type BrokenFella interface {
	RepairTime() float64
	Repair()
	Neighbors(connections ConnectionsGraph) (ns Neighbors)
}

func (t *Train) RepairTime() float64         { return float64(t.repairTime) / 60.0 }
func (tt *Turntable) RepairTime() float64    { return float64(tt.repairTime) / 60.0 }
func (nt *NormalTrack) RepairTime() float64  { return float64(nt.repairTime) / 60.0 }
func (st *StationTrack) RepairTime() float64 { return float64(st.repairTime) / 60.0 }

func (t *Train) Repair()         { t.Repaired <- true }
func (tt *Turntable) Repair()    { tt.Repaired <- true }
func (nt *NormalTrack) Repair()  { nt.Repaired <- true }
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
	id      int // Train's identification
	speed   int // maximum speed in km/h
	station *StationTrack
	at      Track // current position, Track the repair team occupies
	Done    chan bool
}

func NewRepairTeam(id, speed int, station *StationTrack) (team *RepairTeam) {
	team = &RepairTeam{
		id:      id,
		speed:   speed,
		station: station,
		at:      station,
		Done:    make(chan bool)}
	return
}

func (rt *RepairTeam) Simulate(railway *RailwayData, data *SimulationData) {
Loop:
	rt.Station().TeamRider <- rt
	<-rt.Done
	<-rt.Station().Done

	for {
		client := <-railway.RepairChannel
		logger.Printf("%s %v prepares to repair %v",
			ClockTime(data), rt, client)
		destinations := client.Neighbors(railway.Connections)

		for _, d := range destinations {
			if rt.Station() == d {
				logger.Printf("%s %v repairs %v from depot", ClockTime(data), rt, client)
				repairTime := float64(data.SecondsPerHour) * client.RepairTime() * 1000.0
				time.Sleep(time.Duration(repairTime) * time.Millisecond)
				client.Repair()
				goto Loop
			}
		}

		reserved := make([]Track, 0)

		// Reservations
		for _, nt := range railway.NormalTracks {
			if nt == client {
				continue
			} else if nt.Reserve() {
				reserved = append(reserved, nt)
			}
		}
		for _, st := range railway.StationTracks {
			if st == client {
				continue
			} else if st.Reserve() {
				reserved = append(reserved, st)
			}
		}
		for _, tt := range railway.Turntables {
			if tt == client {
				continue
			} else if tt.Reserve() {
				reserved = append(reserved, tt)
			}
		}

		resp := make(chan Path)
		go SearchForPath(Path{rt.Station()}, rt.Station(), destinations, resp, railway.Connections)
		path := <-resp

		logString := fmt.Sprintf("%s %v found path to faulty %v:\n",
			ClockTime(data), rt, client)
		for i, t := range path {
			logString += fmt.Sprintf("%d. %v\n", i, t)
		}

		logger.Print(logString)

	ForAllReserved:
		for _, r := range reserved {
			for _, t := range path {
				if r == t {
					continue ForAllReserved
				}
			}
			r.Cancel()
		}

		for _, track := range path[1:] {
			switch track.(type) {
			case *StationTrack:
				track := track.(*StationTrack)
				track.TeamRider <- rt
				<-track.Done
			case *NormalTrack:
				track := track.(*NormalTrack)
				track.TeamRider <- rt
				<-track.Done
			case *Turntable:
				track := track.(*Turntable)
				track.TeamRider <- rt
				<-track.Done
			}
		}

		logger.Printf("%s %v repairs %v from %v", ClockTime(data), rt, client, path[len(path)-1])
		repairTime := float64(data.SecondsPerHour) * client.RepairTime() * 1000.0
		time.Sleep(time.Duration(repairTime) * time.Millisecond)
		client.Repair()

		for i := range path[1 : len(path)-1] {
			track := path[len(path)-1-i]
			switch track.(type) {
			case *StationTrack:
				track := track.(*StationTrack)
			Loop1:
				for {
					for _, sibling := range track.Siblings(railway.Connections) {
						st := sibling.(*StationTrack)
						select {
						case st.TeamRider <- rt:
							<-st.Done
							break Loop1
						default:
							continue
						}
					}
				}
			case *NormalTrack:
				track := track.(*NormalTrack)
			Loop2:
				for {
					for _, sibling := range track.Siblings(railway.Connections) {
						nt := sibling.(*NormalTrack)
						select {
						case nt.TeamRider <- rt:
							<-nt.Done
							break Loop2
						default:
							continue
						}
					}
				}
			case *Turntable:
				track := track.(*Turntable)
				track.TeamRider <- rt
				<-track.Done
			}
		}

		rt.Station().TeamRider <- rt
		<-rt.Station().Done
		logger.Printf("%s %v returned to depot", ClockTime(data), rt)
	}
}

func (rt *RepairTeam) Station() *StationTrack { return rt.station }
func (rt *RepairTeam) Speed() int             { return rt.speed }
func (rt *RepairTeam) At() Track              { return rt.at }
func (rt *RepairTeam) SetAt(at Track)         { rt.at = at }

// String returns human-friendly label for Train t
func (rt *RepairTeam) String() string { return fmt.Sprintf("RepairTeam%d", rt.id) }

// GoString returns more verbose human-friendly representation of Train t
func (rt *RepairTeam) GoString() string {
	return fmt.Sprintf(
		"rails.RepairTeam:%d{speed:%d, station:%s, at:%s}",
		rt.id, rt.speed, rt.station, rt.at)
}

func SearchForPath(currentPath Path, from Track, destination Neighbors, resp chan Path, graph ConnectionsGraph) {
	for _, track := range from.Neighbors(graph) {
		if track.isAvailable() {
			for _, d := range destination {
				if track == d {
					select {
					case resp <- append(currentPath, track):
						return
					default:
						return
					}
				}
			}
			SearchForPath(append(currentPath, track), track, destination, resp, graph)
		}
	}
}
