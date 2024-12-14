#!/bin/bash

#go mod init github.com/guybal/tarbac


go mod tidy            # Ensure dependencies are up to date
go build -o bin/manager main.go