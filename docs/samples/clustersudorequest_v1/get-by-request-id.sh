#!/usr/bin/env bash

requestId=$1
echo
kubectl get sudorequest,temporaryrbac,rolebinding -A -l tarbac.io/request-id=$requestId
echo
#kubectl get events -A | grep $requestId
echo
kubectl get clustersudorequest,clustertemporaryrbac,clusterrolebinding -l tarbac.io/request-id=$requestId
echo