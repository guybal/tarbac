#!/bin/bash

controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./..."
go build -o bin/manager main.go
