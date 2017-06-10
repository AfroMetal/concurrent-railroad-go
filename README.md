# Concurrent railroad simulator in Go #

### Compilation: ###
`go build main.go`

### Usage: ###
`./main [FLAGS]`

or

`go run main.go [FLAGS]`

where options are:
```
   -d    generate Graphviz .dot file of railroad
   -i string
         input file containing railroad description (default "input")
   -o string
         output file for statistics saving, will be overwritten (default "output")
   -r    simulate breakage and repair using RepairTeams
   -v    print state changes in real time
   -w    simulate Workers and jobs dispatcher

```

#### Configuration file: ####
Configuration file defines railroad characteristics such as:
* simulation start clock,
* how many seconds should hour simulation take,
* specification of all tracks: turntables, station and normal tracks,
* specification of trains together with their route,
* specification of repair teams,

Example configuration file can be found in `input` with further instructions on how to write such file.
