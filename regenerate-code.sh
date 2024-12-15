#!/bin/bash

controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./api/v1"
go build -o bin/manager main.go
