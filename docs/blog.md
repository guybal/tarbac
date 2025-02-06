# 🚀 TARBAC: IT Breaks-the-Glass for Self-Service Dynamic Access

In the fast-paced world of Kubernetes, static Role-Based Access Control (RBAC) systems often fall short. Traditional RBAC models are rigid, requiring manual intervention for role assignments and cleanups, which can lead to security risks and operational inefficiencies. Enter Tarbac, a cloud native solution designed to address these challenges by introducing time-based and self-service, policy-driven access controls.

**Temporal Auditable Role Based Access Controller (TARBAC)** provides a Kubernetes-native solution to manage temporary RBAC permissions dynamically. It ensures secure, time-limited access by leveraging a self-service, policy-driven approach. Developers request what they need, policies validate the request, and temporary access is granted (and revoked) automatically.

## 🔍 The Problems

### 🌍 Static Permissions in a Dynamic World

Traditional RBAC systems are static, meaning roles are created once and persist indefinitely unless manually revoked. This approach is not ideal for dynamic Kubernetes environments where access needs can change rapidly. Static permissions can lead to over-privileged users and potential security vulnerabilities and sometimes are just not enough.

**Tarbac Solution**: Time-bound bindings. Tarbac introduces bindings that automatically expire after a specified duration.
This ensures that access is granted just in time, for just the right amount of time, and disappears automatically, eliminating the risk of lingering permissions.

### ⏳ The IT Bottleneck

In traditional RBAC systems, IT teams are the gatekeepers of access control. This often results in delays and inefficiencies, especially during critical situations like production outages. Developers need to wait for IT approval to gain the necessary access, which can hinder productivity and slow down incident resolution.

**Tarbac Solution**: Self-service, policy-driven access. Tarbac empowers developers to request access through a self-service model. Policies validate the requests, and access is granted automagically if the request aligns with the predefined policies. This reduces the dependency on IT and accelerates the access provisioning process.

## 🛠️ How Tarbac Works: Key Components

Tarbac leverages six Custom Resource Definitions (CRDs) to manage access control dynamically:

- **`SudoRequest`**: Temporary, namespace-scoped access.
- **`SudoPolicy`**: namespace-scoped policy that validate who can do what, where, and for how long.
- **`ClusterSudoRequest`**: Break-glass access for cluster-wide emergencies.
- **`ClusterSudoPolicy`**: Policies governing cluster-level requests.
- **`TemporaryRBAC`**: Temporary `RoleBindings` for namespace-scoped access.
- **`ClusterTemporaryRBAC`**: Temporary `ClusterRoleBindings` for cluster-scoped access.

## 🔄 The Tarbac Workflow

1. **Request Access**: A user submits a `SudoRequest` or `ClusterSudoRequest` request, specifying nothing but the **policy** to refer to and **duration** requested.
2. **Evaluate User**: The submitting user is fetched at runtime and attached to the request along with a `RequestID`.
3. **Policy Validation**: Tarbac evaluates the request against `SudoPolicy` or `ClusterSudoPolicy`. If the request aligns with the policy, access is granted.
4. **Temporary Bindings**: Tarbac creates the necessary `RoleBindings` or `ClusterRoleBindings` and starts the timer.
5. **Automatic Cleanup**: When the timer expires, the permissions are automatically revoked, ensuring no lingering access.

## 🌐 Real-World Use Cases

### 🐛 Use Case 1: Debugging a Broken Pod in QA

In this scenario, Alice, a QA engineer, needs 30 minutes of edit access in the QA namespace to debug a broken pod.

First, the IT or Security team must define the following `SudoPolicy` beforehand:

```yaml
apiVersion: tarbac.io/v1
kind: SudoPolicy
metadata:
    name: qa-edit-policy
    namespace: qa
spec:
    maxDuration: 1h
    roleRef:
        apiGroup: rbac.authorization.k8s.io
        kind: Role
        name: edit
    allowedUsers:
        - name: alice
```

- **`maxDuration`**: Maximum access duration (e.g., 1 hour).
- **`roleRef`**: References the `Role` to grant (`edit` in this case).
- **`allowedUsers`**: Specifies who can request access.
- **`allowedNamespaces`**: Limits the policy to the `qa` namespace.

Next, Alice submits a `SudoRequest` request in the desired `qa` namespace:

```yaml
apiVersion: tarbac.io/v1
kind: SudoRequest
metadata:
    name: debug-qa-pod
    namespace: qa
spec:
    duration: 30m
    policy: qa-edit-policy
```

- **`duration`**: The duration for which permissions are required (e.g., 30 minutes).
- **`policy`**: The name of the `SudoPolicy` governing the request.

Once approved, Alice gets the access she needs. After 30 minutes, the access is automatically revoked, ensuring no lingering permissions.

### 🚨 Use Case 2: Cluster-Wide Emergency

During a cluster-wide emergency, the lead engineer needs 15 minutes of cluster-admin access.

First, the IT or Security team must define the following `ClusterSudoPolicy` beforehand:

```yaml
apiVersion: tarbac.io/v1
kind: ClusterSudoPolicy
metadata:
    name: emergency-cluster-admin-policy
spec:
    maxDuration: 1h
    roleRef:
        apiGroup: rbac.authorization.k8s.io
        kind: ClusterRole
        name: cluster-admin
    allowedUsers:
        - name: lead-engineer
    allowedNamespaces:
        - '*'
```

- **`maxDuration`**: Maximum access duration (e.g., 1 hour).
- **`roleRef`**: References the `ClusterRole` to grant (`cluster-admin` in this case).
- **`allowedUsers`**: Specifies who can request access.
- **`allowedNamespaces`**: Allows access to all namespaces.

The lead engineer then submits a `ClusterSudoRequest` request:

```yaml
apiVersion: tarbac.io/v1
kind: ClusterSudoRequest
metadata:
    name: emergency-cluster-admin
spec:
    duration: 15m
    policy: emergency-cluster-admin-policy
```

- **`duration`**: The duration for which permissions are required (e.g., 15 minutes).
- **`policy`**: The name of the `ClusterSudoPolicy` governing the request.

The request is validated against the policy, and access is granted. After 15 minutes, the access is automatically revoked, ensuring security and compliance.

### 🌟 Use Case 3: Dynamic Attribute-Based Provisioning

Imagine a development team is having a failure across few services deployed on different namespaces.
Team A's tech lead needs administrative access to all namespaces labeled with `team: a` for an hour.

First, the IT or Security team must define the following `ClusterSudoPolicy` beforehand:

```yaml
apiVersion: tarbac.io/v1
kind: ClusterSudoPolicy
metadata:
    name: team-a-namespaces-admin
spec:
    maxDuration: 4h
    roleRef:
        apiGroup: rbac.authorization.k8s.io
        kind: ClusterRole
        name: cluster-admin
    allowedUsers:
        - name: tech-lead
    allowedNamespacesSelector:
        matchLabels:
            team: a
```

- **`maxDuration`**: Maximum access duration (e.g., 4 hours).
- **`roleRef`**: References the `ClusterRole` to grant (`cluster-admin` in this case).
- **`allowedUsers`**: Specifies who can request access.
- **`allowedNamespacesSelector`**: Limits the policy to namespaces labeled with `team: a`.

The tech lead then submits a `ClusterSudoRequest`:

```yaml
apiVersion: tarbac.io/v1
kind: ClusterSudoRequest
metadata:
    name: break-the-glass
spec:
    duration: 1h
    policy: team-a-namespaces-admin
```

- **`duration`**: The duration for which permissions are required (e.g., 1 hour).
- **`policy`**: The name of the `ClusterSudoPolicy` governing the request.

This policy ensures that the tech lead can only request access to namespaces that are dynamically selected based on the label `team: a`. After the specified duration of 1 hour, the access is automatically revoked, maintaining security and compliance.

### ⚙️ **Administrative View**

#### 🛠️ Runtime Resources

##### 🌳 Resources Hierarchy Tree

The provisioned resources under the request will be in the following construct:

```bash
kubectl tree clustersudorequest.tarbac.io/break-the-glass -A

NAMESPACE  NAME                                                              READY  REASON  AGE
           ClusterSudoRequest/break-the-glass                                -              97s
service-a  ├─TemporaryRBAC/user-tech-lead-team-a-namespaces-222b10e90b91     -              97s
service-a  │ └─RoleBinding/user-tech-lead-cluster-admin-222b10e90b91         -              97s
service-b  ├─TemporaryRBAC/user-tech-lead-team-a-namespaces-222b10e90b91     -              97s
service-b  │ └─RoleBinding/user-tech-lead-cluster-admin-222b10e90b91         -              97s
service-c  └─TemporaryRBAC/user-tech-lead-team-a-namespaces-222b10e90b91     -              97s
service-c    └─RoleBinding/user-tech-lead-cluster-admin-222b10e90b91         -              97s
```

#### 📄 Request Runtime Manifest

The runtime request manifest contains information about the RequestID and provisioned resources:

```bash
apiVersion: tarbac.io/v1
kind: ClusterSudoRequest
metadata:
  annotations:
    tarbac.io/requester: tech-lead
    tarbac.io/requester-metadata: UID=, Groups=[team-a-group system:authenticated]
  creationTimestamp: "2025-02-05T10:47:35Z"
  generation: 1
  name: break-the-glass
  resourceVersion: "73485813"
  uid: ae38f12d-a666-4d60-bb2a-222b10e90b91
spec:
  duration: 1h
  policy: team-a-namespaces-admin
status:
  childResource:
  - apiVersion: tarbac.io/v1
    kind: TemporaryRBAC
    name: user-tech-lead-team-a-namespaces-222b10e90b91
    namespace: service-a
  - apiVersion: tarbac.io/v1
    kind: TemporaryRBAC
    name: user-tech-lead-team-a-namespaces-222b10e90b91
    namespace: service-b
  - apiVersion: tarbac.io/v1
    kind: TemporaryRBAC
    name: user-tech-lead-team-a-namespaces-222b10e90b91
    namespace: service-c
  createdAt: "2025-02-05T10:47:35Z"
  expiresAt: "2025-02-05T11:47:35Z"
  requestID: ae38f12d-a666-4d60-bb2a-222b10e90b91
```

#### 📊 Auditing

Tarbac provides comprehensive auditing capabilities to ensure transparency and accountability. Administrators can track access requests, approvals, and revocations through detailed logs and events. This helps in maintaining a clear audit trail and enhances security by allowing for regular reviews and audits of access activities.

##### 📅 Events

All relevant `Events` are transparently rendered to the cluster, tagged with the corresponding `RequestID`. This ensures a clear audit trail of access requests, approvals, and revocations, providing administrators with detailed insights into who performed what actions, when, and under which policies.

```bash
kubectl get events -n default

LAST SEEN   TYPE     REASON      OBJECT                                             MESSAGE
2h12m       Normal   Submitted   clustersudorequest/break-the-glass   User 'tech-lead' submitted a ClusterSudoRequest for policy 'team-a-namespaces-admin' for a duration of 1h0m0s [RequestID: ae38f12d-a666-4d60-bb2a-222b10e90b91]
2h12m       Normal   Approved    clustersudorequest/break-the-glass   User 'tech-lead' was approved by 'team-a-namespaces-admin' ClusterSudoPolicy [RequestID: ae38f12d-a666-4d60-bb2a-222b10e90b91]
1h12m       Warning  Expired     clustersudorequest/break-the-glass   ClusterSudoRequest of user 'tech-lead' for policy 'team-a-namespaces-admin' expired [RequestID: ae38f12d-a666-4d60-bb2a-222b10e90b91]
```

```bash
kubectl get events -n service-a

LAST SEEN   TYPE     REASON               OBJECT                                                           MESSAGE
2h12m       Normal   PermissionsGranted   temporaryrbac/user-tech-lead-team-a-namespaces-222b10e90b91   Temporary permissions were granted for user-tech-lead-team-a-namespaces-222b10e90b91 in namespace service-a [RequestID: ae38f12d-a666-4d60-bb2a-222b10e90b91]
1h12m       Normal   PermissionsRevoked   temporaryrbac/user-tech-lead-team-a-namespaces-222b10e90b91   Temporary permissions were revoked in namespace service-a [RequestID: ae38f12d-a666-4d60-bb2a-222b10e90b91]
```

##### 📝 Logs

All relevant logs are transparent and sent to standard output coupled with the relevant `RequestID`. This provides a clear audit trail of access requests, ensures accountability and enhances security by allowing administrators to review who requested what, when, and for how long.

```bash
2025-02-05T10:52:35Z    INFO    Checking expiration     {"controller": "temporaryrbac", "controllerGroup": "tarbac.io", "controllerKind": "TemporaryRBAC", "TemporaryRBAC": {"name":"user-test-user-self-service-labeled-222b10e90b91","namespace":"service-c"}, "namespace": "service-c", "name": "user-test-user-self-service-labeled-222b10e90b91", "reconcileID": "bc181bbf-a703-4250-be47-da82723a0d12", "requestID": "ae38f12d-a666-4d60-bb2a-222b10e90b91", "currentTime": "2025-02-05T10:52:35Z", "expiresAt": "2025-02-05 10:52:35 +0000 UTC"}
2025-02-05T10:52:35Z    INFO    TemporaryRBAC expired, cleaning up associated bindings  {"controller": "temporaryrbac", "controllerGroup": "tarbac.io", "controllerKind": "TemporaryRBAC", "TemporaryRBAC": {"name":"user-test-user-self-service-labeled-222b10e90b91","namespace":"service-c"}, "namespace": "service-c", "name": "user-test-user-self-service-labeled-222b10e90b91", "reconcileID": "bc181bbf-a703-4250-be47-da82723a0d12", "requestID": "ae38f12d-a666-4d60-bb2a-222b10e90b91", "currentTime": "2025-02-05T10:52:35Z", "expiresAt": "2025-02-05 10:52:35 +0000 UTC"}
2025-02-05T10:52:35Z    INFO    TemporaryRBAC status updated    {"controller": "temporaryrbac", "controllerGroup": "tarbac.io", "controllerKind": "TemporaryRBAC", "TemporaryRBAC": {"name":"user-test-user-self-service-labeled-222b10e90b91","namespace":"service-c"}, "namespace": "service-c", "name": "user-test-user-self-service-labeled-222b10e90b91", "reconcileID": "bc181bbf-a703-4250-be47-da82723a0d12", "requestID": "ae38f12d-a666-4d60-bb2a-222b10e90b91", "kind": "TemporaryRBAC", "name": "user-test-user-self-service-labeled-222b10e90b91", "namespace": "service-c", "state": "Expired"}
```

This detailed view provides administrators with a clear understanding of the resources provisioned under the request, ensuring transparency and accountability in access management. By maintaining a comprehensive audit trail and offering real-time monitoring, Tarbac enables administrators to effectively manage and review access activities, thereby enhancing overall security.

## 🔮 The Future of RBAC: Why Tarbac Really Shines

### 🚀 **Dynamic Access Control**

Tarbac aligns access control with the dynamic nature of Kubernetes environments, ensuring that permissions are granted just in time and revoked automatically.

### ⚡ **Self-Service Model**

By decoupling IT from access management, Tarbac empowers developers to request and gain access quickly, improving productivity and reducing operational bottlenecks.

### 🔒 **Enhanced Security and Compliance**

By ensuring that permissions are time-bound and automatically revoked, Tarbac minimizes the risk of over-privileged users and potential security vulnerabilities.

### ☁️ **Cloud Native Purpose Built**

Tarbac is designed specifically for cloud-native environments, leveraging Kubernetes-native constructs to provide seamless integration and operation within Kubernetes clusters.

### 📜 **Transparent Auditing in Logs and Eventing**

Tarbac ensures that all access requests and actions are transparently logged and can be audited. This provides a clear trail of who accessed what, when, and for how long, enhancing accountability and security.

## 🗂️ Backlog Features

### 🛡️ **Enhanced Policy Capabilities**

- Support for more complex policy rules.

### 📊 **Monitoring**

- Expose Prometheus metrics to monitor the health and performance of Tarbac components.
- Suggested metrics include the number of active requests, policies usage, evaluation times, and the number of expired permissions.
- Create Grafana dashboards to visualize these metrics and provide insights into access control activities.

### 📝 **Sudo History**

- Implement a feature to view the audit log from the kube-apiserver, detailing actions performed by users with elevated permissions.
- This track log will help administrators understand what changes were made during the elevated access period, enhancing transparency and accountability.

## 📄 Summary

Tarbac revolutionizes Kubernetes access control by introducing dynamic, time-bound, and self-service RBAC permissions. It addresses the limitations of traditional static RBAC systems, reducing security risks and operational inefficiencies. By empowering developers with policy-driven access requests and ensuring automatic revocation of permissions, Tarbac enhances productivity, security, and compliance in cloud-native environments. With comprehensive auditing and monitoring capabilities, it provides transparency and accountability, making it an essential tool for modern Kubernetes access management.
