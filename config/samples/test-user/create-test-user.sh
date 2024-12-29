#!/bin/bash

# Variables
USER_NAME="test-user"
GROUP_NAME="testing-group"
NAMESPACE="default"
CLUSTER_NAME="aks-tap-build"
API_SERVER="https://aks-tap-build-c5qsl1ps.hcp.eastus.azmk8s.io:443" # Replace with your Kubernetes API server address
CA_CERT_PATH="ca.crt"

# Generate private key and CSR
echo "Generating private key and CSR..."
openssl genrsa -out ${USER_NAME}.key 2048
openssl req -new -key ${USER_NAME}.key -out ${USER_NAME}.csr -subj "/CN=${USER_NAME}/O=${GROUP_NAME}"

# Create a Kubernetes CSR resource
echo "Creating CSR in the cluster..."
cat <<EOF | kubectl apply -f -
apiVersion: certificates.k8s.io/v1
kind: CertificateSigningRequest
metadata:
  name: ${USER_NAME}-csr
spec:
  request: $(cat ${USER_NAME}.csr | base64 | tr -d '\n')
  signerName: kubernetes.io/kube-apiserver-client
  usages:
  - client auth
EOF

# Approve the CSR
echo "Approving the CSR..."
kubectl certificate approve ${USER_NAME}-csr

# Fetch the signed certificate
echo "Fetching the signed certificate..."
kubectl get csr ${USER_NAME}-csr -o jsonpath='{.status.certificate}' | base64 -d > ${USER_NAME}.crt

# Create a Role
echo "Creating Role..."
cat <<EOF | kubectl apply -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  namespace: $NAMESPACE
  name: sudo-request-role
rules:
- apiGroups: ["tarbac.io"]
  resources: ["sudorequests"]
  verbs: ["create", "get", "list", "watch", "delete"]
EOF

# Create a RoleBinding
echo "Creating RoleBinding..."
cat <<EOF | kubectl apply -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  namespace: $NAMESPACE
  name: ${USER_NAME}-binding
subjects:
- kind: User
  name: $USER_NAME
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: Role
  name: sudo-request-role
  apiGroup: rbac.authorization.k8s.io
EOF

# Create a Role
echo "Creating ClusterRole..."
cat <<EOF | kubectl apply -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: sudo-request-cluster-role
rules:
- apiGroups: ["tarbac.io"]
  resources: ["clustersudorequests"]
  verbs: ["create", "get", "list", "watch", "delete"]
EOF

# Create a RoleBinding
echo "Creating ClusterRoleBinding..."
cat <<EOF | kubectl apply -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: ${USER_NAME}-cluster-binding
subjects:
- kind: User
  name: $USER_NAME
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: sudo-request-cluster-role
  apiGroup: rbac.authorization.k8s.io
EOF

# Create kubeconfig for the test user
echo "Creating kubeconfig file..."
kubectl config set-cluster $CLUSTER_NAME \
  --server=$API_SERVER \
  --certificate-authority=$CA_CERT_PATH \
  --kubeconfig=${USER_NAME}.kubeconfig

kubectl config set-credentials $USER_NAME \
  --client-certificate=${USER_NAME}.crt \
  --client-key=${USER_NAME}.key \
  --kubeconfig=${USER_NAME}.kubeconfig

kubectl config set-context ${USER_NAME}-context \
  --cluster=$CLUSTER_NAME \
  --namespace=$NAMESPACE \
  --user=$USER_NAME \
  --kubeconfig=${USER_NAME}.kubeconfig

kubectl config use-context ${USER_NAME}-context --kubeconfig=${USER_NAME}.kubeconfig

# Verify the setup
echo "Verifying the kubeconfig setup..."
KUBECONFIG=${USER_NAME}.kubeconfig kubectl get sudorequests

echo "Test user '${USER_NAME}' created successfully. Use '${USER_NAME}.kubeconfig' for authentication."
