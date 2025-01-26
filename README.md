# Time and Role Based Access Controller

- [Time and Role Based Access Controller](#time-and-role-based-access-controller)
  - [Install using Helm](#install-using-helm)
    - [Create a Registry Secret](#create-a-registry-secret)
    - [Prepare `tarbac-values.yaml` File](#prepare-tarbac-valuesyaml-file)
    - [Install](#install)

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
