#!/bin/bash

docker build -t docker.io/guybalmas/temporary-rbac-controller:v1.0.13 .
docker login
docker push docker.io/guybalmas/temporary-rbac-controller:v1.0.13

kubectl delete -f /mnt/c/Users/GuyBalmas/IdeaProjects/teraky-int/tarbac-controller/config/manager
kubectl apply -f /mnt/c/Users/GuyBalmas/IdeaProjects/teraky-int/tarbac-controller/config/manager
