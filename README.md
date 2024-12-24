# Time and Role Based Access Controller

## Prerequisites 

## Install

### Install using Helm

##### Prepare `tarbac-values.yaml` File
```yaml
image:
  repository: docker.io/guybalmas/temporary-rbac-controller
  tag: v1.1.2
  pullSecret:
    name: dockerhub-creds       

namespace:
  create: false # If already created
```

##### Install 
```bash

helm install tarbac config/helm -f tarbac-values.yaml -n temporary-rbac-controller
```

##### Test
```bash
kubectl apply -f config/samples/sudorequest_v1/example.yaml

bash config/samples/sudorequest_v1/verify.sh
```
**Output**:
```bash
> View runtime YAML manifest for SudoPolicy Rresource:
apiVersion: tarbac.io/v1
kind: SudoPolicy
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"tarbac.io/v1","kind":"SudoPolicy","metadata":{"annotations":{},"name":"self-service-namespace-admin","namespace":"default"},"spec":{"allowedUsers":[{"name":"masterclient"}],"maxDuration":"4h","roleRef":{"apiGroup":"rbac.authorization.k8s.io","kind":"ClusterRole","name":"cluster-admin"}}}
  creationTimestamp: "2024-12-24T20:43:09Z"
  generation: 1
  name: self-service-namespace-admin
  namespace: default
  resourceVersion: "29492269"
  uid: ffbca01d-8963-4c57-8f79-a8bcecfebe03
spec:
  allowedUsers:
  - name: masterclient
  maxDuration: 4h
  roleRef:
    apiGroup: rbac.authorization.k8s.io
    kind: ClusterRole
    name: cluster-admin
status:
  state: Active


> View runtime YAML manifest for SudoRequest resource:
apiVersion: tarbac.io/v1
kind: SudoRequest
metadata:
  annotations:
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"tarbac.io/v1","kind":"SudoRequest","metadata":{"annotations":{},"name":"example-sudo-request","namespace":"default"},"spec":{"duration":"1m","policy":"self-service-namespace-admin"}}
    tarbac.io/requester: masterclient
    tarbac.io/requester-metadata: UID=, Groups=[system:masters system:authenticated]
  creationTimestamp: "2024-12-24T20:43:10Z"
  generation: 1
  name: example-sudo-request
  namespace: default
  resourceVersion: "29492297"
  uid: 3c1ccc4e-c7d9-4742-b491-1bbd36f21c82
spec:
  duration: 1m
  policy: self-service-namespace-admin
status:
  childResource:
  - apiVersion: tarbac.io/v1
    kind: TemporaryRBAC
    name: temporaryrbac-example-sudo-request
    namespace: default
  createdAt: "2024-12-24T20:43:10Z"
  expiresAt: "2024-12-24T20:44:10Z"
  requestID: 3c1ccc4e-c7d9-4742-b491-1bbd36f21c82
  state: Approved


> View runtime YAML manifest for TemporaryRBAC resource
apiVersion: tarbac.io/v1
kind: TemporaryRBAC
metadata:
  creationTimestamp: "2024-12-24T20:43:10Z"
  generation: 1
  name: temporaryrbac-example-sudo-request
  namespace: default
  ownerReferences:
  - apiVersion: tarbac.io/v1
    blockOwnerDeletion: true
    controller: true
    kind: SudoRequest
    name: example-sudo-request
    uid: 3c1ccc4e-c7d9-4742-b491-1bbd36f21c82
  resourceVersion: "29492298"
  uid: 1143dadf-409c-479e-bfa7-2926d1948eb9
spec:
  duration: 1m
  retentionPolicy: retain
  roleRef:
    apiGroup: rbac.authorization.k8s.io
    kind: ClusterRole
    name: cluster-admin
  subjects:
  - kind: User
    name: masterclient
status:
  childResource:
  - apiVersion: rbac.authorization.k8s.io/v1
    kind: RoleBinding
    name: user-masterclient-cluster-admin
    namespace: default
  createdAt: "2024-12-24T20:43:10Z"
  expiresAt: "2024-12-24T20:44:10Z"
  state: Created


> View runtime YAML manifest for RoleBinding resource
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  creationTimestamp: "2024-12-24T20:43:10Z"
  labels:
    tarbac.io/owner: temporaryrbac-example-sudo-request
  name: user-masterclient-cluster-admin
  namespace: default
  ownerReferences:
  - apiVersion: tarbac.io/v1
    blockOwnerDeletion: true
    controller: true
    kind: TemporaryRBAC
    name: temporaryrbac-example-sudo-request
    uid: 1143dadf-409c-479e-bfa7-2926d1948eb9
  resourceVersion: "29492287"
  uid: a3ddfca0-7f73-440a-b931-ed9c73eaf34c
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: User
  name: masterclient
```