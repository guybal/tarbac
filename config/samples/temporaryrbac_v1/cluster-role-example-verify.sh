#!/usr/bin/env bash

echo "Service Account was grant permissions?"
echo "> Can 'default' ServiceAccount list pods in the cluster?"
kubectl auth can-i get pods -n default --as=system:serviceaccount:default:default
echo
echo "> Can 'test' ServiceAccount list secrets in the namespace?"
kubectl auth can-i get pods -n default --as=system:serviceaccount:default:test
echo
echo
echo "View runtime YAML manifest"
kubectl get temporaryrbacs.tarbac.io -n default -o yaml example-multiple-subjects-temporary-rbac
echo
echo
echo "View runtime chile resources"
kubectl get -n default rolebinding serviceaccount-default-get-pods-cluster-role -o yaml
echo
kubectl get -n default rolebinding serviceaccount-test-get-pods-cluster-role -o yaml
echo
echo
echo "View get output:"
kubectl get temporaryrbacs.tarbac.io -n default
echo
echo

