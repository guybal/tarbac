#!/bin/bash

#controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./api/v1"
controller-gen object paths=./api/v1
controller-gen crd paths=./api/v1 output:crd:artifacts:config=./config/crd/bases
go build -o bin/manager main.go
