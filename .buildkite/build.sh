#!/usr/bin/env bash
export GOPATH=~/go
go generate
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -v -a -ldflags '-extldflags \"-static\"' -o gerrit-queue
