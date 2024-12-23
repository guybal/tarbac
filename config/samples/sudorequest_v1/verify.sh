#!/usr/bin/env bash

echo "> View runtime YAML manifest for SudoPolicy Rresource:"
kubectl get sudopolicy -n default self-service-namespace-admin -o yaml
echo
echo
echo "> View runtime YAML manifest for SudoRequest resource:"
kubectl get sudorequest -n default example-sudo-request -o yaml
echo
echo
echo "> View runtime YAML manifest for TemporaryRBAC resource"
kubectl get temporaryrbacs.tarbac.io temporaryrbac-example-sudo-request -n default -o yaml
echo
echo
echo "> View runtime YAML manifest for RoleBinding resource"
kubectl get rolebinding user-masterclient-cluster-admin -n default -o yaml
echo
echo

