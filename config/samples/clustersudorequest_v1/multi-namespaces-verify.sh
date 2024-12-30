#!/usr/bin/env bash

echo "> View runtime YAML manifest for SudoPolicy Rresource:"
kubectl get clustersudopolicy.tarbac.io/self-service-dev-admin -o yaml
echo
echo
echo "> View runtime YAML manifest for SudoRequest resource:"
kubectl get clustersudorequest.tarbac.io/multi-namespace-admin -o yaml
echo
echo
echo "> View runtime YAML manifest for TemporaryRBAC resources:"
echo "'default' namespace:"
kubectl get temporaryrbacs.tarbac.io -n default temporaryrbac-multi-namespace-admin-default -o yaml
echo
echo "'dev' namespace:"
kubectl get temporaryrbacs.tarbac.io -n dev temporaryrbac-multi-namespace-admin-dev -o yaml
echo
echo
echo "> View runtime YAML manifest for RoleBinding resource"
echo "'default' namespace:"
kubectl get rolebinding user-test-user-cluster-admin -n default -o yaml
echo
echo "'dev' namespace:"
kubectl get rolebinding user-test-user-cluster-admin -n default -o yaml
echo
echo

