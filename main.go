/*
 * Radoslaw Kowalski 221454
 */
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"./src/rails"
)

func clockTime() string {
	d := time.Since(start)

	sH, f := math.Modf(d.Seconds() / float64(secondsPerHour))
	sM, f := math.Modf(60.0 * f)
	sS := 60.0 * f

	h := int(sH) + clock.h
	h = h % 24
	m := int(sM) + clock.m
	if m > 59 {
		h++
	}
	m = m % 60
	s := int(sS)

	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

// readFields scans lines until uncommented unempty line, then tokenizes it and returns
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

var secondsPerHour int             // how many seconds one hour of simulation lasts
var clock struct{ h, m int }       // simulation clock start hours and minutes
var start time.Time                // simulation start time for calculating simulation clock
var statisticsWriter *bufio.Writer // statistics fiel writer
var statisticsChannel = make(chan string, 256)

var repairChannel = make(chan rails.BrokenFella)
var connections rails.ConnectionsGraph
var turntables rails.Turntables
var normalTracks rails.NormalTracks
var stationTracks rails.StationTracks
var trains rails.Trains
var repairTeams rails.RepairTeams

var verbose = flag.Bool("verbose", false, "print state changes in real time")
var generateDotFile = flag.Bool("dot", false, "generate Graphviz .dot file of railroad")
var inFilename = flag.String("in", "input", "input file containing railroad description")
var outFilename = flag.String("out", "output", "output file for statistics saving, will be overwritten")

func main() {
	flag.Parse()

	out, err := os.Create(*outFilename)
	check(err)
	defer out.Close()
	statisticsWriter = bufio.NewWriter(out)

	go func() {
		for {
			_, err = statisticsWriter.WriteString(<-statisticsChannel)
			check(err)
			err = statisticsWriter.Flush()
			check(err)
		}
	}()

	in, err := os.Open(*inFilename)
	check(err)
	defer in.Close()
	scan := bufio.NewScanner(in)

	fields, err := readFields(scan, 1)
	check(err)
	sph, err := strconv.Atoi(fields[0])
	check(err)
	secondsPerHour = sph

	fields, err = readFields(scan, 2)
	check(err)
	clock.h, err = strconv.Atoi(fields[0])
	check(err)
	clock.m, err = strconv.Atoi(fields[1])
	check(err)

	fields, err = readFields(scan, 5)
	check(err)
	rts, err := strconv.Atoi(fields[0])
	check(err)
	ts, err := strconv.Atoi(fields[1])
	check(err)
	tts, err := strconv.Atoi(fields[2])
	check(err)
	nts, err := strconv.Atoi(fields[3])
	check(err)
	sts, err := strconv.Atoi(fields[4])
	check(err)

	fmt.Printf("%d Trains\n"+
		"%d turntables\n"+
		"%d normal tracks\n"+
		"%d station tracks\n"+
		"Hour simulation will take %d seconds\n"+
		"Simulation starts at %02d:%02d\n",
		ts, nts, sts, tts, sph, clock.h, clock.m)

	connections = rails.NewConnectionsGraph(tts)

	turntables = make(rails.Turntables, tts)
	for i := range turntables {
		fields, err = readFields(scan, 3)
		check(err)

		id, err := strconv.Atoi(fields[0])
		check(err)
		rTime, err := strconv.Atoi(fields[1])
		check(err)
		repTime, err := strconv.Atoi(fields[2])
		check(err)

		turntables[i] = rails.NewTurntable(id, rTime, repTime)

		go func(self *rails.Turntable) {
			for {
				select {
				case <-self.Broke:
					select {
					case repairChannel <- self:
						log.Printf("%s %v broke", clockTime(), self)
						<-self.Repaired
						log.Printf("%s %v repaired", clockTime(), self)
					default:
						continue
					}
				case <-self.Reserved:
					log.Printf("%s %v is reserved", clockTime(), self)
					select {
					case <-self.Cancelled:
						log.Printf("%s %v reservation cancelled", clockTime(), self)
					case rt := <-self.TeamRider:
						rt.Done <- true

						log.Printf("%s %v rotates at %v",
							clockTime(), rt, self)
						self.Sleep(rt.Speed(), secondsPerHour)
						self.Done <- true
						<-rt.Done
						log.Printf("%s %v reservation cancelled", clockTime(), self)
					}
				case t := <-self.Rider:
					t.Done <- true

					switch t.At().(type) {
					// if train left station save it to timetable
					case *rails.StationTrack:
						statisticsChannel <- fmt.Sprintf("%v %s ->\t%v\n",
							t, clockTime(), t.At())
					}
					// calculate real seconds to simulate action time
					t.SetAt(self)
					log.Printf("%s %v rotates at %v",
						clockTime(), t, self)
					self.Sleep(t.Speed(), secondsPerHour)

					self.Done <- true
					<-t.Done
					if rand.Float64() < 0.08 {
						self.Broke <- self
					}
				case rt := <-self.TeamRider:
					rt.Done <- true

					log.Printf("%s %v rotates at %v",
						clockTime(), rt, self)
					self.Sleep(rt.Speed(), secondsPerHour)

					self.Done <- true
					<-rt.Done
				}
			}
		}(turntables[i])
	}

	normalTracks = make(rails.NormalTracks, nts)
	for i := range normalTracks {
		fields, err = readFields(scan, 6)
		check(err)

		id, err := strconv.Atoi(fields[0])
		check(err)
		len, err := strconv.Atoi(fields[1])
		check(err)
		speed, err := strconv.Atoi(fields[2])
		check(err)
		repTime, err := strconv.Atoi(fields[3])
		check(err)
		fst, err := strconv.Atoi(fields[4])
		check(err)
		snd, err := strconv.Atoi(fields[5])
		check(err)

		normalTracks[i] = rails.NewNormalTrack(id, len, speed, repTime, turntables[fst], turntables[snd])

		connections[fst][snd] = append(connections[fst][snd], normalTracks[i])
		connections[snd][fst] = append(connections[snd][fst], normalTracks[i])

		go func(self *rails.NormalTrack) {
			for {
				select {
				case <-self.Broke:
					select {
					case repairChannel <- self:
						log.Printf("%s %v broke", clockTime(), self)
						<-self.Repaired
						log.Printf("%s %v repaired", clockTime(), self)
					default:
						continue
					}
				case <-self.Reserved:
					log.Printf("%s %v is reserved", clockTime(), self)
					select {
					case <-self.Cancelled:
						log.Printf("%s %v reservation cancelled", clockTime(), self)
					case rt := <-self.TeamRider:
						rt.Done <- true

						log.Printf("%s %v travels along %v",
							clockTime(), rt, self)
						self.Sleep(rt.Speed(), secondsPerHour)

						self.Done <- true
						<-rt.Done
						log.Printf("%s %v reservation cancelled", clockTime(), self)
					}
				case t := <-self.Rider:
					t.Done <- true

					t.SetAt(self)
					log.Printf("%s %v travels along %v",
						clockTime(), t, self)
					self.Sleep(t.Speed(), secondsPerHour)

					self.Done <- true
					<-t.Done
					if rand.Float64() < 0.05 {
						self.Broke <- self
					}
				case rt := <-self.TeamRider:
					rt.Done <- true

					log.Printf("%s %v travels along %v",
						clockTime(), rt, self)
					self.Sleep(rt.Speed(), secondsPerHour)

					self.Done <- true
					<-rt.Done
				}
			}
		}(normalTracks[i])
	}

	stationTracks = make(rails.StationTracks, sts)
	for i := range stationTracks {
		fields, err = readFields(scan, 6)
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

		stationTracks[i] = rails.NewStationTrack(id, name, sTime, repTime, turntables[fst], turntables[snd])

		connections[fst][snd] = append(connections[fst][snd], stationTracks[i])
		if fst != snd {
			connections[snd][fst] = append(connections[snd][fst], stationTracks[i])
		}

		go func(self *rails.StationTrack) {
			for {
				select {
				case <-self.Broke:
					select {
					case repairChannel <- self:
						log.Printf("%s %v broke", clockTime(), self)
						<-self.Repaired
						log.Printf("%s %v repaired", clockTime(), self)
					default:
						continue
					}
				case <-self.Reserved:
					log.Printf("%s %v is reserved", clockTime(), self)
					select {
					case <-self.Cancelled:
						log.Printf("%s %v reservation cancelled", clockTime(), self)
					case rt := <-self.TeamRider:
						rt.Done <- true

						log.Printf("%s %v waits on %v",
							clockTime(), rt, self)
						self.Sleep(rt.Speed(), secondsPerHour)

						self.Done <- true
						<-rt.Done
						log.Printf("%s %v reservation cancelled", clockTime(), self)
					}
				case t := <-self.Rider:
					t.Done <- true

					statisticsChannel <- fmt.Sprintf("%v %s >-\t%v\n",
						t, clockTime(), self)
					// calculate real seconds to simulate action time
					t.SetAt(self)
					log.Printf("%s %v waits on %v",
						clockTime(), t, self)
					self.Sleep(t.Speed(), secondsPerHour)

					self.Done <- true
					<-t.Done
					if rand.Float64() < 0.01 {
						self.Broke <- self
					}
				case rt := <-self.TeamRider:
					rt.Done <- true

					log.Printf("%s %v waits on %v",
						clockTime(), rt, self)
					self.Sleep(rt.Speed(), secondsPerHour)

					self.Done <- true
					<-rt.Done
				}
			}
		}(stationTracks[i])
	}

	if *generateDotFile {
		out, err := os.Create(*outFilename + ".dot")
		check(err)
		defer out.Close()
		dotWriter := bufio.NewWriter(out)

		dotWriter.WriteString(
			fmt.Sprintf("graph %s {graph [pad=\"0.25\", nodesep=\"0.5\", ranksep=\"1.0\"];\n", *inFilename))

		for i := range connections {
			for j := 0; j <= i; j++ {
				for _, t := range connections[i][j] {
					dotWriter.WriteString(
						fmt.Sprintf("\t%d -- %d", i, j))
					switch t.(type) {
					case *rails.StationTrack:
						s, _ := t.(*rails.StationTrack)
						dotWriter.WriteString(
							fmt.Sprintf(" [label=\"%d:%s\", color=blue]\n", s.ID(), s.Name))
					default:
						dotWriter.WriteString(
							fmt.Sprintf(" [label=%d] \n", t.ID()))
					}
				}
				dotWriter.Flush()
			}
		}

		dotWriter.WriteString("}\n")
		dotWriter.Flush()

		fmt.Printf("Graphviz .dot file generated under: %s\n", out.Name())

		os.Exit(0)
	}

	repairTeams = make(rails.RepairTeams, rts)
	for i := range repairTeams {
		fields, err = readFields(scan, 3)
		check(err)

		id, err := strconv.Atoi(fields[0])
		check(err)
		speed, err := strconv.Atoi(fields[1])
		check(err)
		stationId, err := strconv.Atoi(fields[2])
		check(err)

		repairTeams[i] = rails.NewRepairTeam(id, speed, stationTracks[stationId])

		go func(self *rails.RepairTeam) {
		Loop:

			self.Station().TeamRider <- self
			<-self.Done
			<-self.Station().Done

			for {
				client := <-repairChannel
				log.Printf("%s %v prepares to repair %v",
					clockTime(), self, client)
				destinations := client.Neighbors(connections)

				for _, d := range destinations {
					if self.Station() == d {
						log.Printf("%s %v repairs %v from depot", clockTime(), self, client)
						repairTime := float64(sph) * client.RepairTime() * 1000.0
						time.Sleep(time.Duration(repairTime) * time.Millisecond)
						client.Repair()
						goto Loop
					}
				}

				reserved := make([]rails.Track, 0)

				for _, nt := range normalTracks {
					if nt == client {
						continue
					} else if nt.Reserve() {
						reserved = append(reserved, nt)
					}
				}
				for _, st := range stationTracks {
					if st == client {
						continue
					} else if st.Reserve() {
						reserved = append(reserved, st)
					}
				}
				for _, tt := range turntables {
					if tt == client {
						continue
					} else if tt.Reserve() {
						reserved = append(reserved, tt)
					}
				}

				resp := make(chan rails.Path)
				go rails.SearchForPath(rails.Path{self.Station()}, self.Station(), destinations, resp, connections)
				path := <-resp

				logString := fmt.Sprintf("%s %v found path to faulty %v:\n",
					clockTime(), self, client)
				for i, t := range path {
					logString += fmt.Sprintf("%d. %v\n", i, t)
				}

				log.Print(logString)

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
					case *rails.StationTrack:
						track := track.(*rails.StationTrack)
						track.TeamRider <- self
						<-track.Done
					case *rails.NormalTrack:
						track := track.(*rails.NormalTrack)
						track.TeamRider <- self
						<-track.Done
					case *rails.Turntable:
						track := track.(*rails.Turntable)
						track.TeamRider <- self
						<-track.Done
					}
				}

				log.Printf("%s %v repairs %v from %v", clockTime(), self, client, path[len(path)-1])
				repairTime := float64(sph) * client.RepairTime() * 1000.0
				time.Sleep(time.Duration(repairTime) * time.Millisecond)
				client.Repair()

				for i := len(path) - 2; i >= 0; i-- {
					track := path[i]
					switch track.(type) {
					case *rails.StationTrack:
						track := track.(*rails.StationTrack)
					Loop1:
						for {
							for _, sibling := range track.Siblings(connections) {
								st := sibling.(*rails.StationTrack)
								select {
								case st.TeamRider <- self:
									<-st.Done
									break Loop1
								default:
									continue
								}
							}
						}
					case *rails.NormalTrack:
						track := track.(*rails.NormalTrack)
					Loop2:
						for {
							for _, sibling := range track.Siblings(connections) {
								nt := sibling.(*rails.NormalTrack)
								select {
								case nt.TeamRider <- self:
									<-nt.Done
									break Loop2
								default:
									continue
								}
							}
						}
					case *rails.Turntable:
						track := track.(*rails.Turntable)
						track.TeamRider <- self
						<-track.Done
					}
				}

				log.Printf("%s %v returned to depot", clockTime(), self)
			}
		}(repairTeams[i])
	}

	trains = make(rails.Trains, ts)
	for i := range trains {
		fields, err = readFields(scan, 6)
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
		len, err := strconv.Atoi(fields[5])
		check(err)

		fields, err = readFields(scan, len)
		check(err)

		route := rails.Route{}
		for j := 0; j < len; j++ {
			index, err := strconv.Atoi(fields[j])
			check(err)

			route = append(route, turntables[index])
		}

		trains[i] = rails.NewTrain(id, speed, capacity, repTime, name, route)
	}

	waitGroup := new(sync.WaitGroup)
	waitGroup.Add(len(trains))

	if !*verbose {
		log.SetOutput(ioutil.Discard)
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()

			reader := bufio.NewReader(os.Stdin)

			instructions := "Input char for action, availble commands:\n" +
				"\t'c' - simulation clock,\n" +
				"\t'p' - current trains positions,\n" +
				"\ts'ts' - list trains,\n" +
				"\t'u' - list turntables,\n" +
				"\t'n' - list normal tracks,\n" +
				"\t's' - list station tracks,\n" +
				"\t'h' - print this menu again,\n" +
				"\t'v' - enter verbose mode (YOU WILL NOT BE ABLE TO TURN IT OFF),\n" +
				"\t'q' - to quit simulation.\n"
			fmt.Print(instructions)

			for {
				input, err := reader.ReadString('\n')
				check(err)

				r := []rune(input)[0]
				switch unicode.ToUpper(r) {
				case 'C': // clock
					fmt.Println(clockTime())
				case 'P': // positions
					for _, t := range trains {
						fmt.Printf("%v: %v\n", t, t.At())
					}
				case 'T': // trains
					for _, t := range trains {
						fmt.Printf("%#v\n", t)
					}
				case 'U': // turntables
					for _, tt := range turntables {
						fmt.Printf("%#v\n", tt)
					}
				case 'N': // normal tracks
					for _, nt := range normalTracks {
						fmt.Printf("%#v\n", nt)
					}
				case 'S': // station tracks
					for _, st := range stationTracks {
						fmt.Printf("%#v\n", st)
					}
				case 'H': // help
					fmt.Print(instructions)
				case 'V': // verbose
					log.SetOutput(os.Stdout)
					log.SetFlags(0)
					return
				case 'Q': // quit
					os.Exit(0)
				default:
					continue
				}
			}
		}()
	} else {
		log.SetFlags(0)
	}

	start = time.Now()
	for i := range trains {
		go func(t *rails.Train) {
			defer waitGroup.Done()

			log.Printf("%s %v starts work", clockTime(), t)

			track := t.At().(*rails.Turntable)
			track.Rider <- t
			<-t.Done
			<-track.Done

			for {
				select {
				case <-t.Broke:
					select {
					case repairChannel <- t:
						log.Printf("%s %v broke", clockTime(), t)
						<-t.Repaired
						log.Printf("%s %v repaired", clockTime(), t)
					default:
						continue
					}
				default:
					// get nearest Turntables
					fst, snd := t.Connection()
				Loop1: // search for available Track connecting `fst` and `snd`
					for {
						for _, r := range connections[fst.ID()][snd.ID()] {
							switch r.(type) {
							case *rails.StationTrack:
								r := r.(*rails.StationTrack)
								select {
								case r.Rider <- t:
									<-r.Done
									break Loop1
								default:
									continue
								}
							case *rails.NormalTrack:
								r := r.(*rails.NormalTrack)
								select {
								case r.Rider <- t:
									<-r.Done
									break Loop1
								default:
									continue
								}
							}
						}
					}
					snd.Rider <- t
					t.NextPosition()
					<-snd.Done
					if rand.Float64() < 0.005 {
						t.Broke <- t
					}
				}
			}
		}(trains[i])
	}

	waitGroup.Wait()
}
