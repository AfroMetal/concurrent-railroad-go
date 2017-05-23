package rails

import "fmt"

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
