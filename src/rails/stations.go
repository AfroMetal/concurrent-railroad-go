package rails

import (
	"fmt"
	"strings"
)

type Station struct {
	id            int
	Name          string
	first         *Turntable
	second        *Turntable
	StationTracks StationTracks
	Workers       Workers
	Destinations  Stations
}

func NewStation(id int, initial *StationTrack) (station *Station) {
	return &Station{
		id:            id,
		Name:          initial.Name,
		first:         initial.first,
		second:        initial.second,
		StationTracks: StationTracks{initial},
		Workers:       make(Workers, 0),
		Destinations:  make(Stations, 0)}
	return
}

func (s *Station) Connects(first, second *Turntable) bool {
	return (s.first == first && s.second == second) || (s.first == second && s.second == first)
}

type Stations []*Station

func (s *Station) String() string {
	return fmt.Sprintf("Station%d %s", s.id, strings.ToUpper(s.Name))
}
func (s *Station) GoString() string {
	return fmt.Sprintf(
		"rails.Station:%s:%d{turntables: (%s, %s)}",
		s.Name, s.id, s.first, s.second)
}
