#!/usr/bin/env bash

echo "> View runtime YAML manifest for ClusterSudoPolicy Rresource:"
kubectl get clustersudopolicy.tarbac.io/self-service-cluster-admin -o yaml
echo
echo
echo "> View runtime YAML manifest for ClusterSudoRequest resource:"
kubectl get clustersudorequest.tarbac.io/temp-cluster-admin -o yaml
echo
echo
echo "> View runtime YAML manifest for ClusterTemporaryRBAC resources:"
kubectl get clustertemporaryrbacs.tarbac.io cluster-temporaryrbac-temp-cluster-admin -o yaml
echo
echo
echo "> View runtime YAML manifest for ClusterRoleBinding resource"
kubectl get ClusterRoleBinding user-masterclient-cluster-admin -o yaml
echo
echo

