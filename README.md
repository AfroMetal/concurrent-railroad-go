# Concurrent railroad simulator in Go #

### Compilation: ###
`go build main.go`

### Usage: ###
`./main [FLAGS]`

or

`go run main.go [FLAGS]`

where options are:
```
-in filename
    read configuration data from file specified by `filename` (default `input`)
-out filename
    save statistics data to file specified by `filename` (default `input`)
-verbose
    turn verbose mode on (off by default)
-dot
    generate .dot graph file or railway and exit
```

#### Configuration file: ####
Configuration file defines railroad characteristics such as:
* simulation start clock,
* how many seconds should hour simulation take,
* specification of all tracks: turntables, station and normal tracks,
* specification of trains together with their route,
* specification of repair teams,

Example configuration file can be found in `input` with further instructions on how to write such file.
