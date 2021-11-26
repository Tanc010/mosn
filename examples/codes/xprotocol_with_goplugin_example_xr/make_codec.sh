#!/bin/bash

function make_so {
	go build -mod=readonly --buildmode=plugin  -gcflags all="-N -l"  -o codec-xr.so ./codec.go
}

make_so
