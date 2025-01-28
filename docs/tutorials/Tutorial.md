# TARBAC Tutorial: Dynamic, Temporary RBAC for Kubernetes

This tutorial demonstrates how to use **TARBAC** to dynamically assign temporary RBAC permissions in Kubernetes. You’ll learn to define policies, request temporary permissions, and observe how these permissions behave from both a user’s and an admin’s perspective.

By the end, you’ll understand how TARBAC’s CRDs work, how permissions are granted dynamically, and how they automatically expire to reduce the risk of over-provisioned access.

---

## Table of Contents

- [TARBAC Tutorial: Dynamic, Temporary RBAC for Kubernetes](#tarbac-tutorial-dynamic-temporary-rbac-for-kubernetes)
  - [Table of Contents](#table-of-contents)
  - [Prerequisites](#prerequisites)
  - [Use Case: Temporary Cluster-Admin for Labeled Namespaces](#use-case-temporary-cluster-admin-for-labeled-namespaces)
    - [Step 1: Define the Policy](#step-1-define-the-policy)
    - [Step 2: Create a Namespace with Matching Labels](#step-2-create-a-namespace-with-matching-labels)
    - [Step 3: Submit an Access Request](#step-3-submit-an-access-request)
    - [Step 4: Verify and View Temporary Permissions](#step-4-verify-and-view-temporary-permissions)
      - [User Perspective: Consuming Permissions](#user-perspective-consuming-permissions)
      - [Admin Perspective: Monitoring Permissions](#admin-perspective-monitoring-permissions)
    - [Step 5: Automatic Expiry](#step-5-automatic-expiry)
      - [User Perspective: Post-Expiry](#user-perspective-post-expiry)
      - [Admin Perspective: Post-Expiry](#admin-perspective-post-expiry)
  - [FAQ](#faq)
    - [What happens if I request a duration longer than the policy allows?](#what-happens-if-i-request-a-duration-longer-than-the-policy-allows)
    - [Can I apply this to multiple users?](#can-i-apply-this-to-multiple-users)
    - [What happens if no namespaces match the label selector?](#what-happens-if-no-namespaces-match-the-label-selector)

---

## Prerequisites

Before starting, ensure the following:

- A Kubernetes cluster with TARBAC controllers installed.
  - [Install TARBAC using Helm](https://github.com/guybal/tarbac/blob/main/README.md#install-using-helm)
- `kubectl` configured to interact with your cluster.
- `test-user`: A testing user authenticated to your cluster with the necessary permissions for TARBAC resources (`ClusterSudoRequest`, `SudoRequest`).  
  - You can set this up using the `create-test-user.sh` script available in the TARBAC repository, which configures a user and provides the required permissions for testing.

---

## Use Case: Temporary Cluster-Admin for Labeled Namespaces

In this example, we’ll create a `ClusterSudoPolicy` that allows `test-user` to request temporary `cluster-admin` permissions for namespaces labeled with `team: a`. The flow will demonstrate defining policies, submitting requests, and verifying access.

---

### Step 1: Define the Policy

The `ClusterSudoPolicy` governs the rules for temporary access:

```yaml
apiVersion: tarbac.io/v1
kind: ClusterSudoPolicy
metadata:
  name: team-a-namespaces
spec:
  maxDuration: 4h
  roleRef:
    apiGroup: rbac.authorization.k8s.io
    kind: ClusterRole
    name: cluster-admin
  allowedUsers:
    - name: test-user # Replace with your user
  allowedNamespacesSelector:
    matchLabels:
      team: a
```

- **`maxDuration`**: Maximum access duration (e.g., 4 hours).
- **`roleRef`**: References the `ClusterRole` to grant (`cluster-admin` in this case).
- **`allowedUsers`**: Specifies who can request access.
- **`allowedNamespacesSelector`**: Limits the policy to namespaces labeled with `team: a`.

**Apply the policy**:

```bash
kubectl apply -f cluster-sudo-policy.yaml
```

Expected output:

```bash
clustersudopolicy.tarbac.io/team-a-namespaces created
```

At this stage, you have defined the policy that will govern how permissions are granted. The policy acts as the enforcement mechanism, ensuring that requests exceeding the defined duration or not matching the namespace labels are denied.

---

### Step 2: Create a Namespace with Matching Labels

Create a namespace that fits the policy’s selector. This step ensures that the namespace where you want to apply temporary permissions aligns with the labels specified in the `allowedNamespacesSelector`.

```bash
kubectl create namespace example-namespace
kubectl label namespace example-namespace team=a
```

Expected output:

```bash
namespace/example-namespace created
namespace/example-namespace labeled
```

This namespace, labeled with `team=a`, will now match the policy's `allowedNamespacesSelector`, making it eligible for temporary permissions.

---

### Step 3: Submit an Access Request

Create a `ClusterSudoRequest` to request temporary `cluster-admin` access for the namespace. This step simulates a user requesting elevated permissions dynamically.

```yaml
apiVersion: tarbac.io/v1
kind: ClusterSudoRequest
metadata:
  name: my-namespace-admin
spec:
  duration: 5m
  policy: team-a-namespaces
```

- **`duration`**: The duration for which permissions are required (e.g., 5 minutes).
- **`policy`**: The name of the `ClusterSudoPolicy` governing the request.

**Apply the request**:

```bash
kubectl apply -f cluster-sudo-request.yaml
```

Expected output:

```bash
clustersudorequest.tarbac.io/my-namespace-admin created
```

By applying this request, the TARBAC controllers will process it based on the defined policy and dynamically create the necessary RBAC bindings.

---

### Step 4: Verify and View Temporary Permissions

This step is divided into two perspectives: the **user** who submitted the request and the **admin** who manages the cluster.

---

#### User Perspective: Consuming Permissions

1. **Verify Access**:
   Attempt to list pods in the namespace using the temporary permissions:

   ```bash
   kubectl get pods -n example-namespace --as=test-user
   ```

   Expected output:

   ```bash
   No resources found in example-namespace namespace
   ```

   Even though no pods are present, the command demonstrates that the `test-user` can now interact with resources in the namespace.

2. **List Active Requests**:
   View all active `ClusterSudoRequest` resources:

   ```bash
   kubectl get clustersudorequests.tarbac.io --as=test-user
   ```

   Expected output:

   ```bash
   NAME                 STATE      DURATION   CREATED AT             EXPIRES AT
   my-namespace-admin   Approved   5m         2025-01-26T21:20:05Z   2025-01-26T21:25:05Z
   ```

   This output shows that the request is in the `Approved` state and provides details about its validity period.

3. **Inspect the Request**:
   Get detailed information about the request:

   ```bash
   kubectl get clustersudorequests.tarbac.io my-namespace-admin -o yaml --as=test-user
   ```

   Expected output:

   ```yaml
   apiVersion: tarbac.io/v1
   kind: ClusterSudoRequest
   metadata:
     annotations:
       kubectl.kubernetes.io/last-applied-configuration: |
         {"apiVersion":"tarbac.io/v1","kind":"ClusterSudoRequest","metadata":{"annotations":{},"name":"my-namespace-admin"},"spec":{"duration":"5m","policy":"team-a-namespaces"}}
       tarbac.io/requester: test-user
       tarbac.io/requester-metadata: UID=, Groups=[testing-group system:authenticated]
     creationTimestamp: "2025-01-26T21:20:05Z"
     generation: 1
     name: my-namespace-admin
     resourceVersion: "63894054"
     uid: d64b6fa5-c691-4a1d-aa2f-614804be9726
   spec:
     duration: 5m
     policy: team-a-namespaces
   status:
     childResource:
     - apiVersion: tarbac.io/v1
       kind: TemporaryRBAC
       name: user-test-user-team-a-namespaces-614804be9726
       namespace: example-namespace
     createdAt: "2025-01-26T21:20:05Z"
     expiresAt: "2025-01-26T21:25:05Z"
     requestID: d64b6fa5-c691-4a1d-aa2f-614804be9726
     state: Approved
   ```

---

#### Admin Perspective: Monitoring Permissions

1. **View the Permission Hierarchy**:
   As an admin, check the resource tree created by the request:

   ```bash
   kubectl tree clustersudorequest my-namespace-admin -A
   ```

   Expected output:

   ```bash
   NAMESPACE          NAME                                                           READY  REASON  AGE
                      ClusterSudoRequest/my-namespace-admin                          -              72s
   example-namespace  └─TemporaryRBAC/user-test-user-team-a-namespaces-614804be9726  -              72s
   example-namespace    └─RoleBinding/user-test-user-cluster-admin-614804be9726      -              71s
   ```

   This output provides a clear hierarchical view of the resources created by the `ClusterSudoRequest`.

2. **Monitor Events**:
   View events in the default namespace:

   ```bash
   kubectl get events -n default
   ```

   Expected output:

   ```bash
   LAST SEEN   TYPE     REASON               OBJECT                                   MESSAGE
   4m16s       Normal   Submitted            clustersudorequest/my-namespace-admin    User test-user submitted a ClusterSudoRequest for policy team-a-namespaces
   4m16s       Normal   Approved             clustersudorequest/my-namespace-admin    User 'test-user' was approved by 'team-a-namespaces' ClusterSudoPolicy [UID: d64b6fa5-c691-4a1d-aa2f-614804be9726]
   ```

3. **Namespace-Level Events**:
   Check events in the target namespace:

   ```bash
   kubectl get events -n example-namespace
   ```

   Expected output:

   ```bash
   LAST SEEN   TYPE     REASON               OBJECT                                                        MESSAGE
   4m30s       Normal   PermissionsGranted   temporaryrbac/user-test-user-team-a-namespaces-614804be9726   Temporary permissions were granted in namespace example-namespace [UID: d64b6fa5-c691-4a1d-aa2f-614804be9726]
   ```

---

### Step 5: Automatic Expiry

When the duration expires, TARBAC automatically revokes permissions. Here’s how this process looks:

---

#### User Perspective: Post-Expiry

1. **Verify Access Revocation**:
   Attempt to list resources after expiry:

   ```bash
   kubectl get pods -n example-namespace --as=test-user
   ```

   Expected output:

   ```bash
   Error from server (Forbidden): pods is forbidden: User "test-user" cannot list resource "pods"
   ```

2. **View Expired Requests**:
   Check the status of expired requests:

   ```bash
   kubectl get clustersudorequests.tarbac.io --as=test-user
   ```

   Expected output:

   ```bash
   NAME                 STATE     DURATION   CREATED AT             EXPIRES AT
   my-namespace-admin   Expired   5m         2025-01-26T21:20:05Z   2025-01-26T21:25:05Z
   ```

---

#### Admin Perspective: Post-Expiry

1. **Monitor Events**:
   View events indicating permission revocation:

   ```bash
   kubectl get events -n default
   ```

   Expected output:

   ```bash
   LAST SEEN   TYPE      REASON               OBJECT                                                        MESSAGE
   30m         Normal    Submitted            clustersudorequest/my-namespace-admin                         User test-user submitted a ClusterSudoRequest for policy team-a-namespaces for a duration of 5m0s [UID: d64b6fa5-c691-4a1d-aa2f-614804be9726]
   30m         Normal    Approved             clustersudorequest/my-namespace-admin                         User 'test-user' was approved by 'team-a-namespaces' ClusterSudoPolicy [UID: d64b6fa5-c691-4a1d-aa2f-614804be9726]
   25m         Warning   Expired              clustersudorequest/my-namespace-admin                         ClusterSudoRequest of user 'test-user' for policy 'team-a-namespaces' expired [UID: d64b6fa5-c691-4a1d-aa2f-614804be9726]
   ```

2. **Namespace-Level Events**:
   Check events in the target namespace:

   ```bash
   kubectl get events -n example-namespace
   ```

   Expected output:

   ```bash
   LAST SEEN   TYPE     REASON               OBJECT                                                        MESSAGE
   52m         Normal   PermissionsGranted   temporaryrbac/user-test-user-team-a-namespaces-614804be9726   Temporary permissions were granted in namespace example-namespace [UID: d64b6fa5-c691-4a1d-aa2f-614804be9726]
   52m         Normal   PermissionsRevoked   temporaryrbac/user-test-user-team-a-namespaces-614804be9726   Temporary permissions were revoked in namespace example-namespace [UID: d64b6fa5-c691-4a1d-aa2f-614804be9726]
   ```

---

## FAQ

### What happens if I request a duration longer than the policy allows?

The request will be denied. Ensure the `duration` is within the `maxDuration` defined in the policy.

### Can I apply this to multiple users?

Yes, add more usernames under `allowedUsers` in the `ClusterSudoPolicy`.

### What happens if no namespaces match the label selector?

The request will be denied, as there are no eligible namespaces.
