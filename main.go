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
	"math/rand"
	"os"
	"sync"
	"time"
	"unicode"

	"./src/rails"
)

func check(e error) {
	if e != nil {
		panic(e)
	}
}

var statisticsWriter *bufio.Writer // statistics file writer
var statisticsChannel = make(chan string, 256)
var logger *log.Logger

var railway *rails.RailwayData = &rails.RailwayData{RepairChannel: make(chan rails.BrokenFella)}
var data *rails.SimulationData = &rails.SimulationData{StatisticsChannel: &statisticsChannel}

var verbose = flag.Bool("v", false, "print state changes in real time")
var generateDotFile = flag.Bool("d", false, "generate Graphviz .dot file of railroad")
var inFilename = flag.String("i", "input", "input file containing railroad description")
var outFilename = flag.String("o", "output", "output file for statistics saving, will be overwritten")
var simulateRepairs = flag.Bool("r", false, "simulate breakage and repair using RepairTeams")
var simulateWorkers = flag.Bool("w", false, "simulate Workers and jobs dispatcher")

func main() {
	rand.Seed(time.Now().UnixNano())
	flag.Parse()

	data.SimulateRepairs = *simulateRepairs
	data.SimulateWorkers = *simulateWorkers

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

	data.Parse(scan)
	railway.Parse(scan)

	fmt.Printf("%v\n", data)
	fmt.Printf("%v\n", railway)

	// DOT FILE
	if *generateDotFile {
		out, err := os.Create(*outFilename + ".dot")
		check(err)
		defer out.Close()
		dotWriter := bufio.NewWriter(out)

		dotWriter.WriteString(
			fmt.Sprintf("graph %s {graph [pad=\"0.25\", nodesep=\"0.5\", ranksep=\"1.0\"];\n", *inFilename))

		for i := range railway.Connections {
			for j := 0; j <= i; j++ {
				for _, t := range railway.Connections[i][j] {
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

	waitGroup := new(sync.WaitGroup)
	waitGroup.Add(len(railway.Trains))

	// VERBOSE MODE
	if !*verbose {
		logger = log.New(ioutil.Discard, "", 0)
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()

			reader := bufio.NewReader(os.Stdin)

			instructions := "Input char for action, available commands:\n" +
				"\t'c' - simulation clock,\n" +
				"\t'p' - current trains positions,\n" +
				"\t't' - list trains,\n" +
				"\t'r' - list repair teams,\n" +
				"\t'u' - list turntables,\n" +
				"\t'n' - list normal tracks,\n" +
				"\t's' - list stations with station tracks,\n" +
				"\t'w' - list workers,\n" +
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
					fmt.Println(rails.ClockTime(data))
				case 'P': // positions
					for _, t := range railway.Trains {
						fmt.Printf("%v: %v\n", t, t.At())
					}
					for _, rt := range railway.RepairTeams {
						fmt.Printf("%v: %v\n", rt, rt.At())
					}
				case 'T': // trains
					for _, t := range railway.Trains {
						fmt.Printf("%v, position: %v, connects:\n", t, t.At())
						for _, s := range t.Connects {
							fmt.Printf("\t%v\n", s)
						}
					}
				case 'R': // repair teams
					if data.SimulateRepairs {
						for _, rt := range railway.RepairTeams {
							fmt.Printf("%v, position: %v\n", rt, rt.At())
						}
					} else {
						fmt.Println("Repair simulation is OFF")
					}
				case 'U': // turntables
					for _, tt := range railway.Turntables {
						fmt.Printf("%v\n", tt)
					}
				case 'N': // normal tracks
					for _, nt := range railway.NormalTracks {
						fmt.Printf("%v\n", nt)
					}
				case 'S': // stations
					for _, s := range railway.Stations {
						fmt.Printf("%v:\n", s)
						for _, st := range s.StationTracks {
							fmt.Printf("\t%v\n", st)
						}
					}
				case 'W': // workers
					if data.SimulateWorkers {
						for _, w := range railway.Workers {
							var position string
							if w.In != nil {
								position = fmt.Sprintf("travels by %v", w.In)
							} else if w.At != nil {
								if w.At == w.Home && w.Job.Workplace == nil {
									position = "is resting at home"
								} else {
									position = fmt.Sprintf("waits at %v", w.At)
								}
							}
							fmt.Printf("%v %s\n", w, position)
						}
					} else {
						fmt.Println("Workers simulation is OFF")
					}
				case 'H': // help
					fmt.Print(instructions)
				case 'V': // verbose
					logger = log.New(os.Stdout, "", 0)
					return
				case 'Q': // quit
					os.Exit(0)
				default:
					continue
				}
			}
		}()
	} else {
		logger = log.New(os.Stdout, "", 0)
	}

	rails.Simulate(railway, data, logger, waitGroup)

	waitGroup.Wait()
}
