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

var repairChannel = make(chan rails.BrokenFella, 256)
var connections rails.Connections
var turntables []*rails.Turntable
var normalTracks []*rails.NormalTrack
var stationTracks []*rails.StationTrack
var trains []*rails.Train
var repairTeams []*rails.RepairTeam

var verbose = flag.Bool("verbose", false, "print state changes in real time")
var generateDotFile = flag.Bool("dot", false, "generate Graphviz .dot file of railroad")
var inFilename = flag.String("in", "input", "input file containing railroad description")
var outFilename = flag.String("out", "output", "output file for statistics saving, will be overwritten")

func search(currentPath rails.Path, from rails.BrokenFella, destination rails.Neighbors) rails.Path {
	for _, track := range from.Neighbors(connections) {
		switch track.(type) {
		case *rails.Turntable:
			track := track.(*rails.Turntable)
			select {
			case <-track.Available:
				for _, d := range destination {
					if track == d {
						return append(currentPath, track)
					}
				}
				return search(append(currentPath, track), track, destination)
			default:
				continue
			}
		case *rails.NormalTrack:
			track := track.(*rails.NormalTrack)
			select {
			case <-track.Available:
				for _, d := range destination {
					if track == d {
						return append(currentPath, track)
					}
				}
				return search(append(currentPath, track), track, destination)
			default:
				continue
			}
		case *rails.StationTrack:
			track := track.(*rails.StationTrack)
			select {
			case <-track.Available:
				for _, d := range destination {
					if track == d {
						return append(currentPath, track)
					}
				}
				return search(append(currentPath, track), track, destination)
			default:
				continue
			}
		}
	}
	return currentPath
}

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

	connections = make([]map[int][]rails.Track, tts)
	for i := range connections {
		connections[i] = make(map[int][]rails.Track)
		for j := range connections[i] {
			connections[i][j] = []rails.Track{}
		}
	}

	turntables = make([]*rails.Turntable, tts)
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
					repairChannel <- self
					<-self.Repaired
				case <-self.Reserved:
					log.Printf("%s %v is reserved", clockTime(), self)
					<-self.Cancelled
					log.Printf("%s %v reservation cancelled", clockTime(), self)
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
					//ts.Delay(time)
					self.Sleep(t.Speed(), secondsPerHour)

					self.Done <- true
					<-t.Done
					if rand.Float64() < 0.08 {
						self.Broke <- self
						log.Printf("%s %v broke", clockTime(), self)
					}
				}
			}
		}(turntables[i])
	}

	normalTracks = make([]*rails.NormalTrack, nts)
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
					repairChannel <- self
					<-self.Repaired
				case <-self.Reserved:
					log.Printf("%s %v is reserved", clockTime(), self)
					<-self.Cancelled
					log.Printf("%s %v reservation cancelled", clockTime(), self)
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
						log.Printf("%s %v broke", clockTime(), self)
					}
				}
			}
		}(normalTracks[i])
	}

	stationTracks = make([]*rails.StationTrack, sts)
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
					repairChannel <- self
					<-self.Repaired
				case <-self.Reserved:
					log.Printf("%s %v is reserved", clockTime(), self)
					<-self.Cancelled
					log.Printf("%s %v reservation cancelled", clockTime(), self)
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
						log.Printf("%s %v broke", clockTime(), self)
					}
				}
			}
		}(stationTracks[i])
	}

	if *generateDotFile {
		out, err := os.Create(*outFilename + ".gv")
		check(err)
		defer out.Close()
		dotWriter := bufio.NewWriter(out)

		dotWriter.WriteString("graph " + *inFilename + " {" +
			"graph [pad=\"0.25\", nodesep=\"0.5\", ranksep=\"1.0\"];\n")

		for i := range connections {
			for j := range connections[i] {
				for _, t := range connections[i][j] {
					dotWriter.WriteString("\t" + strconv.Itoa(i) + " -- " + strconv.Itoa(j))
					switch t.(type) {
					case *rails.StationTrack:
						s, _ := t.(*rails.StationTrack)
						dotWriter.WriteString(" [label=" + s.Name + ", color=blue]\n")
						dotWriter.WriteString("\t{rank=same; " + strconv.Itoa(i) + "; " + strconv.Itoa(j) + "}\n")
					default:
						dotWriter.WriteString("\n")
					}
				}
				dotWriter.Flush()
			}
		}

		dotWriter.WriteString("}\n")
		dotWriter.Flush()

		fmt.Printf("Graphviz .gv file generated under: %s\n", out.Name())

		os.Exit(0)
	}

	repairTeams = make([]*rails.RepairTeam, rts)
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
			for {
				client := <-repairChannel
				destinations := client.Neighbors(connections)
				for _, d := range destinations {
					if self.Station() == d {
						log.Printf("%s %v repairs %v from depot", clockTime(), self, client)
						repairTime := float64(sph) * client.RepairTime() * 1000.0
						time.Sleep(time.Duration(repairTime) * time.Millisecond)
						client.Repair()
						log.Printf("%s %v repaired", clockTime(), client)
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
					if st == client || st == self.Station() {
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

				path := search(rails.Path{self.Station().Neighbors(connections)[0]}, client, destinations)
				for i, p := range path {
					log.Printf("%d) %v\n", i, p)
				}

			ForAllReserved:
				for _, r := range reserved {
					for _, t := range path {
						if r == t {
							continue ForAllReserved
						}
					}
					r.Cancel()
				}

				for _, track := range path {
					log.Printf("%s %v is on %v", clockTime(), self, track)
					track.Sleep(self.Speed(), secondsPerHour)
				}

				log.Printf("%s %v repairs %v from %v", clockTime(), self, client, path[len(path)-1])
				repairTime := float64(sph) * client.RepairTime() * 1000.0
				time.Sleep(time.Duration(repairTime) * time.Millisecond)

				client.Repair()
				log.Printf("%s %v repaired", clockTime(), client)

				for i := len(path) - 1; i >= 0; i-- {
					log.Printf("%s %v backs-up on %v", clockTime(), self, path[i])
					path[i].Sleep(self.Speed(), secondsPerHour)
					path[i].Cancel()
				}
				log.Printf("%s %v returned to depot", clockTime(), self)
			}
		}(repairTeams[i])
	}

	trains = make([]*rails.Train, ts)
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
					repairChannel <- t
					<-t.Repaired
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
						log.Printf("%s %v broke", clockTime(), t)
					}
				}
			}
		}(trains[i])
	}

	waitGroup.Wait()
}
