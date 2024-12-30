#!/usr/bin/env bash

requestId=$1
echo
kubectl get sudorequest,temporaryrbac,rolebinding -n default -l tarbac.io/request-id=$requestId
echo
