#!/usr/bin/env bash

docker build . -t selfsigned-ca-injector:latest
docker tag selfsigned-ca-injector:latest docker.io/guybalmas/selfsigned-ca-injector:latest
docker push docker.io/guybalmas/selfsigned-ca-injector:latest