# Time and Role Based Access Controller

- [Time and Role Based Access Controller](#time-and-role-based-access-controller)
  - [Design Overview](#design-overview)
  - [Install using Helm](#install-using-helm)
    - [Create a Registry Secret](#create-a-registry-secret)
    - [Prepare `tarbac-values.yaml` File](#prepare-tarbac-valuesyaml-file)
    - [Install](#install)
  - [Getting Started with TARBAC](#getting-started-with-tarbac)

**Time and Role-Based Access Controller (TARBAC)** provides a Kubernetes-native solution to manage temporary RBAC permissions dynamically. It ensures secure, time-limited access by leveraging a self-service, policy-driven approach. Developers request what they need, policies validate the request, and temporary access is granted (and revoked) automatically.

## Design Overview

TARBAC is designed to provide dynamic, temporary RBAC permissions in Kubernetes. It leverages Custom Resource Definitions (CRDs) and controllers to manage policies and requests for temporary access. The system ensures secure, time-limited access by validating requests against predefined policies and automatically revoking permissions after their expiration.

For a detailed design document, please refer to:
[**TARBAC High-Level Design**](./docs/design.md)

---

## Install using Helm

### Create a Registry Secret

```bash
DOCKER_REGISTRY_SERVER=docker.io
DOCKER_USER=<Type your dockerhub username, same as when you `docker login`>
DOCKER_PASSWORD=<Type your dockerhub password, same as when you `docker login`>

kubectl create secret docker-registry dockerhub-creds \
  --docker-server=$DOCKER_REGISTRY_SERVER \
  --docker-username=$DOCKER_USER \
  --docker-password=$DOCKER_PASSWORD \
```

### Prepare `tarbac-values.yaml` File

```yaml
image:
  # repository: docker.io/guybalmas/temporary-rbac-controller
  # tag: v1.1.10
  pullSecret:
    name: dockerhub-creds
```

### Install

```bash

helm install tarbac config/helm -f tarbac-values.yaml -n tarbac-system --create-namespace
```

---

## Getting Started with TARBAC

Learn how to use **TARBAC** to dynamically manage temporary RBAC permissions in your Kubernetes cluster.
The tutorial walks you through defining policies, requesting temporary permissions, and verifying how permissions are granted and automatically revoked after their duration expires.

The tutorial includes practical, step-by-step examples, including:

- Setting up policies.
- Dynamically consuming temporary RBAC roles.
- Observing TARBAC behavior from both user and admin perspectives.
- Automatically revoking permissions and cleaning up resources.

You can find the full tutorial here:  
[**TARBAC Tutorial: Dynamic, Temporary RBAC for Kubernetes**](./docs/tutorials/Tutorial.md)

This tutorial is ideal for developers, platform engineers, and cluster administrators looking to implement temporary, time-based RBAC in Kubernetes clusters.
