/*
 * Radoslaw Kowalski 221454
 */
package rails

import (
	"fmt"
	"strings"
	"sync"
)

type Station struct {
	id            int
	Name          string
	first         *Turntable
	second        *Turntable
	Residents     WorkerSlice
	Trains        TrainSlice
	StationTracks StationTrackSlice
	TicketsFor    map[*Train]Tickets
	ticketsMutex  sync.Mutex
	Destinations  StationSlice
}

func NewStation(id int, initial *StationTrack) (station *Station) {
	return &Station{
		id:            id,
		Name:          initial.Name,
		first:         initial.first,
		second:        initial.second,
		Residents:     make(WorkerSlice, 0),
		Trains:        make(TrainSlice, 0),
		StationTracks: StationTrackSlice{initial},
		TicketsFor:    make(map[*Train]Tickets, 0),
		Destinations:  make(StationSlice, 0)}
}

func (s *Station) ID() int { return s.id }

func (s *Station) Connects(first, second *Turntable) bool {
	return (s.first == first && s.second == second) || (s.first == second && s.second == first)
}

type StationSlice []*Station

func (s *Station) String() string {
	return fmt.Sprintf("Station%d %s", s.id, strings.ToUpper(s.Name))
}
func (s *Station) GoString() string {
	return fmt.Sprintf(
		"rails.Station:%s:%d{turntables: (%s, %s)}",
		s.Name, s.id, s.first, s.second)
}
