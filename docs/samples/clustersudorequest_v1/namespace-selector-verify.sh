#!/usr/bin/env bash

echo "> View runtime YAML manifest for SudoPolicy Rresource:"
kubectl get clustersudopolicy.tarbac.io/self-service-labeled-namespaces-admin -o yaml
echo
echo
echo "> View runtime YAML manifest for SudoRequest resource:"
kubectl get clustersudorequest.tarbac.io/multi-labeled-namespace-admin -o yaml
echo
echo
echo "> View runtime YAML manifest for TemporaryRBAC resources:"
echo "'apps' namespace:"
kubectl get temporaryrbacs.tarbac.io -n apps temporaryrbac-multi-labeled-namespace-admin-apps -o yaml
echo
echo "'test' namespace:"
kubectl get temporaryrbacs.tarbac.io -n test temporaryrbac-multi-labeled-namespace-admin-test -o yaml
echo
echo
echo "> View runtime YAML manifest for RoleBinding resource"
echo "'apps' namespace:"
kubectl get rolebinding user-masterclient-cluster-admin -n apps -o yaml
echo
echo "'test' namespace:"
kubectl get rolebinding user-masterclient-cluster-admin -n test -o yaml
echo
echo

