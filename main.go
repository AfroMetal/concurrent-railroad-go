package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"rails"
)

// Ride simulates Train t actions on railroad defined by it's route and connections
func Ride(t *rails.Train, connections *[]map[int][]rails.Track, group *sync.WaitGroup) {
	defer group.Done()

	for {
		fst, snd := t.Connection()
	Loop1:
		for {
			for _, r := range (*connections)[fst.ID()][snd.ID()] {
				switch r.GetLock() {
				case true:
					switch r.(type) {
					case *rails.StationTrack:
						statisticsChannel <- fmt.Sprintf("%v\t%s >-\t%v\n",
							t, simulationNow(), r)
					}
					time := float64(secondsPerHour) * t.MoveTo(r)
					log.Printf("%s\t%v travels along %v",
						simulationNow(), t, r)
					t.SleepSeconds(time)
					break Loop1
				case false:
					continue
				}
			}
			time := float64(secondsPerHour) * 0.25
			log.Printf("%s\t%v have nowhere to go, it will wait for %.2fs",
				simulationNow(), t, time)
			t.SleepSeconds(time)
		}
	Loop2:
		for {
			switch snd.GetLock() {
			case true:
				switch t.At().(type) {
				case *rails.StationTrack:
					statisticsChannel <- fmt.Sprintf("%v\t%s ->\t%v\n",
						t, simulationNow(), t.At())
				}
				time := float64(secondsPerHour) * t.MoveTo(snd)
				log.Printf("%s\t%v rotates at %v",
					simulationNow(), t, snd)
				t.SleepSeconds(time)
				break Loop2
			case false:
				time := float64(secondsPerHour) * 0.25
				log.Printf("%s\t%v have nowhere to go, it will wait for %.2fs",
					simulationNow(), t, time)
				t.SleepSeconds(time)
				continue
			}
		}
	}
}

func simulationNow() string {
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

var secondsPerHour int
var clock struct {
	h, m int
}
var start time.Time
var statisticsWriter *bufio.Writer
var statisticsChannel = make(chan string, 256)

var verbose = flag.Bool("verbose", false, "print state changes in real time")
var inFilename = flag.String("in", "input", "input file containing railroad description")
var outFilename = flag.String("out", "output", "output file for statistics saving, will be overwritten")

func main() {
	flag.Parse()
	start = time.Now()

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

	fields, err = readFields(scan, 4)
	check(err)
	t, err := strconv.Atoi(fields[0])
	check(err)
	tt, err := strconv.Atoi(fields[1])
	check(err)
	nt, err := strconv.Atoi(fields[2])
	check(err)
	st, err := strconv.Atoi(fields[3])
	check(err)

	fmt.Printf("%d Trains\n"+
		"%d turntables\n"+
		"%d normal tracks\n"+
		"%d station tracks\n"+
		"Hour simulation will take %d seconds\n"+
		"Simulation starts at %02d:%02d\n",
		t, nt, st, tt, sph, clock.h, clock.m)

	turntables := make([]*rails.Turntable, tt)
	normalTracks := make([]*rails.NormalTrack, nt)
	stationTracks := make([]*rails.StationTrack, st)
	trains := make([]*rails.Train, t)

	connections := make([]map[int][]rails.Track, tt)
	for i := range connections {
		connections[i] = make(map[int][]rails.Track)
		for j := range connections[i] {
			connections[i][j] = []rails.Track{}
		}
	}

	for i := range turntables {
		fields, err := readFields(scan, 2)
		check(err)

		id, err := strconv.Atoi(fields[0])
		check(err)
		time, err := strconv.Atoi(fields[1])
		check(err)

		turntables[i] = rails.NewTurntable(id, time)
	}

	for i := range normalTracks {
		fields, err := readFields(scan, 5)
		check(err)

		id, err := strconv.Atoi(fields[0])
		check(err)
		len, err := strconv.Atoi(fields[1])
		check(err)
		speed, err := strconv.Atoi(fields[2])
		check(err)
		fst, err := strconv.Atoi(fields[3])
		check(err)
		snd, err := strconv.Atoi(fields[4])
		check(err)

		normalTracks[i] = rails.NewNormalTrack(id, len, speed)

		connections[fst][snd] = append(connections[fst][snd], normalTracks[i])
		connections[snd][fst] = append(connections[snd][fst], normalTracks[i])
	}

	for i := range stationTracks {
		fields, err := readFields(scan, 5)
		check(err)

		id, err := strconv.Atoi(fields[0])
		check(err)
		name := fields[1]
		time, err := strconv.Atoi(fields[2])
		check(err)
		fst, err := strconv.Atoi(fields[3])
		check(err)
		snd, err := strconv.Atoi(fields[4])
		check(err)

		stationTracks[i] = rails.NewStationTrack(id, name, time)

		connections[fst][snd] = append(connections[fst][snd], stationTracks[i])
		connections[snd][fst] = append(connections[snd][fst], stationTracks[i])
	}

	waitGroup := new(sync.WaitGroup)
	waitGroup.Add(len(trains))

	for i := range trains {
		fields, err := readFields(scan, 5)
		check(err)

		id, err := strconv.Atoi(fields[0])
		check(err)
		speed, err := strconv.Atoi(fields[1])
		check(err)
		capacity, err := strconv.Atoi(fields[2])
		check(err)
		name := fields[3]
		len, err := strconv.Atoi(fields[4])
		check(err)

		fields, err = readFields(scan, len)
		check(err)

		route := rails.Route{}
		for j := 0; j < len; j++ {
			index, err := strconv.Atoi(fields[j])
			check(err)

			route = append(route, turntables[index])
		}

		trains[i] = rails.NewTrain(id, speed, capacity, name, route)
		go Ride(trains[i], &connections, waitGroup)
	}

	if !*verbose {
		log.SetOutput(ioutil.Discard)
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()

			reader := bufio.NewReader(os.Stdin)

			instructions := "Input char for action, availble commands:\n" +
				"\t'c' - simulation clock,\n" +
				"\t'p' - current trains positions,\n" +
				"\t't' - list trains,\n" +
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
					fmt.Println(simulationNow())
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

	waitGroup.Wait()
}
