# Reload

A project to re-run any command based on changes in files on a glob pattern

**THIS IS AN INTERNAL EARLY ALPHA PROJECT! PLEASE DO NOT USE IT**

## Getting started

 - Download the binary
 - Run eg. `./reload -w "./**/" go run main.go`

### Usage:

`reload [command]`

#### Flags:

flag   | type   | description
-------|--------|-----------------------
-debug | bool   | enable verbose output
-w     | string | glob pattern to watch
 