package rails

import (
	"bufio"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
)

var logger *log.Logger

type SimulationData struct {
	SecondsPerHour    int                // how many seconds one hour of simulation lasts
	clock             struct{ h, m int } // simulation clock start hours and minutes
	Start             time.Time          // simulation start time for calculating simulation clock
	StatisticsChannel *chan string
}

func (d *SimulationData) Parse(scan *bufio.Scanner) {
	fields, err := readFields(scan, 1)
	check(err)
	sph, err := strconv.Atoi(fields[0])
	check(err)
	d.SecondsPerHour = sph

	fields, err = readFields(scan, 2)
	check(err)
	h, err := strconv.Atoi(fields[0])
	check(err)
	m, err := strconv.Atoi(fields[1])
	check(err)
	d.clock.h = h
	d.clock.m = m
}

func (d *SimulationData) String() string {
	return fmt.Sprintf(
		"hour takes %d seconds\n"+
			"simulation start %02d:%02d",
		d.SecondsPerHour, d.clock.h, d.clock.m)
}

func ClockTime(data *SimulationData) string {
	d := time.Since(data.Start)

	sH, f := math.Modf(d.Seconds() / float64(data.SecondsPerHour))
	sM, f := math.Modf(60.0 * f)
	sS := 60.0 * f

	h := int(sH) + data.clock.h
	h = h % 24
	m := int(sM) + data.clock.m
	if m > 59 {
		h++
	}
	m = m % 60
	s := int(sS)

	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

type RailwayData struct {
	rts, ts, tts, nts, sts, ws int
	Connections                ConnectionsGraph
	Turntables                 TurntableSlice
	NormalTracks               NormalTrackSlice
	StationTracks              StationTrackSlice
	Trains                     TrainSlice
	RepairTeams                RepairTeamSlice
	RepairChannel              chan BrokenFella
	Stations                   StationSlice
	Workers                    WorkerSlice
}

func (r *RailwayData) String() string {
	return fmt.Sprintf("%d trains\n"+
		"%d repair trains\n"+
		"%d turntables\n"+
		"%d normal tracks\n"+
		"%d station tracks\n"+
		"%d workers",
		r.ts, r.rts, r.tts, r.nts, r.sts, r.ws)
}

// readFields scans lines until uncommented non-empty line, then tokenize it and returns
func readFields(scan *bufio.Scanner, expected int) ([]string, error) {
	scan.Scan()
	text := scan.Text()
	for strings.HasPrefix(text, "#") || text == "" {
		scan.Scan()
		text = scan.Text()
	}
	fields := strings.Fields(text)
	if l := len(fields); l != expected {
		return fields, fmt.Errorf("expected to read %d fields, found %d", expected, l)
	}
	return fields, nil
}

func (r *RailwayData) Parse(scan *bufio.Scanner) {
	fields, err := readFields(scan, 6)
	check(err)
	r.rts, err = strconv.Atoi(fields[0])
	check(err)
	r.ts, err = strconv.Atoi(fields[1])
	check(err)
	r.tts, err = strconv.Atoi(fields[2])
	check(err)
	r.nts, err = strconv.Atoi(fields[3])
	check(err)
	r.sts, err = strconv.Atoi(fields[4])
	check(err)
	r.ws, err = strconv.Atoi(fields[5])

	r.Connections = NewConnectionsGraph(r.tts)
	r.Turntables = make(TurntableSlice, r.tts)
	r.NormalTracks = make(NormalTrackSlice, r.nts)
	r.StationTracks = make(StationTrackSlice, r.sts)
	r.Trains = make(TrainSlice, r.ts)
	r.RepairTeams = make(RepairTeamSlice, r.rts)
	r.Workers = make(WorkerSlice, r.ws)
	r.Stations = make(StationSlice, 0)

	r.parseTurntables(scan)
	r.parseNormalTracks(scan)
	r.parseStationTracks(scan)
	r.createStations()
	r.parseRepairTeams(scan)
	r.parseTrains(scan)
	r.parseWorkers(scan)
}

func (r *RailwayData) parseTurntables(scan *bufio.Scanner) {
	for i := range r.Turntables {
		fields, err := readFields(scan, 3)
		check(err)

		id, err := strconv.Atoi(fields[0])
		check(err)
		rTime, err := strconv.Atoi(fields[1])
		check(err)
		repTime, err := strconv.Atoi(fields[2])
		check(err)

		r.Turntables[i] = NewTurntable(id, rTime, repTime)
	}
}

func (r *RailwayData) parseNormalTracks(scan *bufio.Scanner) {
	for i := range r.NormalTracks {
		fields, err := readFields(scan, 6)
		check(err)

		id, err := strconv.Atoi(fields[0])
		check(err)
		length, err := strconv.Atoi(fields[1])
		check(err)
		speed, err := strconv.Atoi(fields[2])
		check(err)
		repTime, err := strconv.Atoi(fields[3])
		check(err)
		fst, err := strconv.Atoi(fields[4])
		check(err)
		snd, err := strconv.Atoi(fields[5])
		check(err)

		r.NormalTracks[i] = NewNormalTrack(id, length, speed, repTime, r.Turntables[fst], r.Turntables[snd])

		r.Connections[fst][snd] = append(r.Connections[fst][snd], r.NormalTracks[i])
		r.Connections[snd][fst] = append(r.Connections[snd][fst], r.NormalTracks[i])
	}
}

func (r *RailwayData) parseStationTracks(scan *bufio.Scanner) {
	for i := range r.StationTracks {
		fields, err := readFields(scan, 6)
		check(err)

		id, err := strconv.Atoi(fields[0])
		check(err)
		name := fields[1]
		sTime, err := strconv.Atoi(fields[2])
		check(err)
		repTime, err := strconv.Atoi(fields[3])
		check(err)
		fst, err := strconv.Atoi(fields[4])
		check(err)
		snd, err := strconv.Atoi(fields[5])
		check(err)

		r.StationTracks[i] = NewStationTrack(id, name, sTime, repTime, r.Turntables[fst], r.Turntables[snd])

		r.Connections[fst][snd] = append(r.Connections[fst][snd], r.StationTracks[i])
		if fst != snd {
			r.Connections[snd][fst] = append(r.Connections[snd][fst], r.StationTracks[i])
		}
	}
}

func (r *RailwayData) parseRepairTeams(scan *bufio.Scanner) {
	for i := range r.RepairTeams {
		fields, err := readFields(scan, 3)
		check(err)

		id, err := strconv.Atoi(fields[0])
		check(err)
		speed, err := strconv.Atoi(fields[1])
		check(err)
		stationId, err := strconv.Atoi(fields[2])
		check(err)

		r.RepairTeams[i] = NewRepairTeam(id, speed, r.StationTracks[stationId])
	}
}

func (r *RailwayData) parseTrains(scan *bufio.Scanner) {
	for i := range r.Trains {
		fields, err := readFields(scan, 6)
		check(err)

		id, err := strconv.Atoi(fields[0])
		check(err)
		speed, err := strconv.Atoi(fields[1])
		check(err)
		capacity, err := strconv.Atoi(fields[2])
		check(err)
		repTime, err := strconv.Atoi(fields[3])
		check(err)
		name := fields[4]
		length, err := strconv.Atoi(fields[5])
		check(err)

		fields, err = readFields(scan, length)
		check(err)

		route := Route{}
		for j := 0; j < length; j++ {
			index, err := strconv.Atoi(fields[j])
			check(err)

			route = append(route, r.Turntables[index])
		}

		train := NewTrain(id, speed, capacity, repTime, name, route)

		prev := route[len(route)-1]
		for i := 0; i < len(route)-1; i++ {
			next := route[i]
			for _, s := range r.Stations {
				if s.Connects(prev, next) {
					train.Connects = append(train.Connects, s)
					s.Trains = append(s.Trains, train)
					s.TicketsFor[train] = make(Tickets, 0)
				}
			}
			prev = next
		}

		r.Trains[i] = train
	}
}

func (r *RailwayData) parseWorkers(scan *bufio.Scanner) {
	for i := range r.Workers {
		fields, err := readFields(scan, 2)
		check(err)

		id, err := strconv.Atoi(fields[0])
		check(err)
		home, err := strconv.Atoi(fields[1])
		check(err)

		station := r.StationTracks[home].Station()
		r.Workers[i] = NewWorker(id, station)
		station.Residents = append(station.Residents, r.Workers[i])
	}
}

func (r *RailwayData) createStations() {
CreateStations:
	for _, st := range r.StationTracks {
		for _, station := range r.Stations {
			if st.BelongsTo(station) {
				station.StationTracks = append(station.StationTracks, st)
				st.SetStation(station)
				continue CreateStations
			}
		}
		station := NewStation(len(r.Stations), st)
		r.Stations = append(r.Stations, station)
		st.SetStation(station)
	}
}

func Simulate(railway *RailwayData, data *SimulationData, log *log.Logger, wg *sync.WaitGroup) {
	logger = log

	// START SIMULATION
	data.Start = time.Now()
	// TURNTABLES
	for _, t := range railway.Turntables {
		go t.Simulate(railway, data)
	}
	// NORMAL TRACKS
	for _, nt := range railway.NormalTracks {
		go nt.Simulate(railway, data)
	}
	// STATION TRACKS
	for _, st := range railway.StationTracks {
		go st.Simulate(railway, data)
	}
	// REPAIR TEAMS
	//for _, rt := range railway.RepairTeams {
	//	go rt.Simulate(railway, data)
	//}
	// TRAINS
	for _, t := range railway.Trains {
		go t.Simulate(railway, data, wg)
	}
	// WORKERS
	for _, w := range railway.Workers {
		go w.Simulate(railway, data)
	}
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}
