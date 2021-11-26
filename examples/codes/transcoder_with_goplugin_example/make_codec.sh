#!/bin/bash

function make_so {
	go build -mod=readonly --buildmode=plugin  -gcflags all="-N -l"  -o xr2http.so ./xr2http.go
}

make_so
