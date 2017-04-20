# Concurrent railroad simulator in Go #
### Requirements ###
Go requires that each used package is available under `$GOPATH` or `$GOROOT` path.
Add `rails` package to your environment variables before compilation.

### Compilation: ###
`go build main.go`

### Usage: ###
`./main [FLAGS]`

where options are:
```
-in filename
    read configuration data from file specified by `filename` (default `input`)
-out filename
    save statistics data to file specified by `filename` (default `input`)
-verbose
    turn verbose mode on (off by default)
```

#### Configuration file: ####
Configuration file defines railroad characteristics such as:
* simulation start clock,
* how many seconds should hour simulation take,
* specification of all tracks: turntables, station and normal tracks,
* specification of trains together with their route.

Example configuration file can be found in `input` with further instructions on how to write such file.