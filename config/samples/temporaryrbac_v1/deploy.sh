#!/usr/bin/env bash

kubectl apply -f test-cluster-role.yaml
kubectl apply -f test-role.yaml
kubectl apply -f multiple_subects.yaml