# Time and Role Based Access Controller

- [Time and Role Based Access Controller](#time-and-role-based-access-controller)
  - [Install](#install)
    - [TL;DR](#tldr)
      - [Installation Script](#installation-script)
      - [Helm](#helm)
    - [Detailed Installation](#detailed-installation)
      - [Prerequisites](#prerequisites)
      - [Installation](#installation)
        - [View Available Values](#view-available-values)
        - [Create `tarbac-values.yaml` file](#create-tarbac-valuesyaml-file)
  - [Design Overview](#design-overview)
  - [Getting Started with TARBAC](#getting-started-with-tarbac)

**Time, Atribute & Role-Based Access Controller (TARBAC)** provides a Kubernetes-native solution to manage temporary RBAC permissions dynamically. It ensures secure, time-limited access by leveraging a self-service, policy-driven approach. Developers request what they need, policies validate the request, and temporary access is granted (and revoked) automatically.

---

## Install

### TL;DR

#### Installation Script

To quickly install TARBAC, run the following script:

```bash
curl -sL https://raw.githubusercontent.com/guybal/tarbac/main/install.sh | bash
```

#### Helm

Alternatively, you can install TARBAC using Helm:

```bash
helm install tarbac oci://ghcr.io/guybal/helm-charts/tarbac --version 1.1.6 --namespace tarbac-system --create-namespace
```

### Detailed Installation

#### Prerequisites

To install TARBAC, ensure you have the following prerequisites:

- A running [**Kubernetes cluster**](https://kubernetes.io/docs/setup/): Version `v1.29.10` or later.
- [**cert-manager**](https://cert-manager.io/docs/installation/helm/): Version `v1.16.2` or later installed on the Kubernetes cluster.
  
    **⚠️ Warning**
    > Disabling `cert-manager` integration for certificate management is supported and will result in a self-signed certificate for the Webhook. However, it is **highly recommended** to use `cert-manager` to ensure secure and automated certificate handling.

- [**kubectl**](https://kubernetes.io/docs/tasks/tools/#kubectl): Client and Server Version `v1.29.10` or later.
- [**Go**](https://go.dev/doc/install): Version `1.23.0` or later.

#### Installation

##### View Available Values

```bash
helm show values oci://ghcr.io/guybal/helm-charts/tarbac --version 1.1.6
```

##### Create `tarbac-values.yaml` file

Create a `tarbac-values.yaml` file to customize the installation:

```yaml
namespace:
  name: tarbac-system
image:
  repository: ghcr.io/guybal/tarbac/controller
  tag: v1.1.14
```

Use this file during the Helm installation:

```bash
helm install tarbac oci://ghcr.io/guybal/helm-charts/tarbac --version 1.1.6 -f tarbac-values.yaml --namespace tarbac-system --create-namespace
```

---

## Design Overview

TARBAC is designed to provide dynamic, temporary RBAC permissions in Kubernetes. It leverages Custom Resource Definitions (CRDs) and controllers to manage policies and requests for temporary access. The system ensures secure, time-limited access by validating requests against predefined policies and automatically revoking permissions after their expiration.

For a detailed design document, please refer to:
[**TARBAC High-Level Design**](./docs/design.md)

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
