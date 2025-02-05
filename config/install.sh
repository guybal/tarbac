#!/bin/bash

# Variables
HELM_CHART="ghcr.io/guybal/helm-charts/tarbac"
VERSION="1.1.7"
RELEASE_NAME="tarbac"
NAMESPACE="tarbac-system" 

# Check if Helm is installed
if ! command -v helm &> /dev/null
then
    echo "Helm could not be found, please install Helm first."
    exit 1
fi

# Create tarbac-values.file
cat <<EOF > tarbac-values.yaml
# tarbac-values.yaml
namespace:
  name: $NAMESPACE
image:
  repository: ghcr.io/guybal/tarbac/controller
  tag: v1.1.15
EOF

# Install the Helm chart
helm install $RELEASE_NAME oci://$HELM_CHART --version $VERSION --namespace $NAMESPACE --create-namespace --values tarbac-values.yaml
