### **Step 1: Initialize Your Environment**
Ensure the following tools are installed:
- **Docker** (for building the image)
- **kubectl** (for interacting with the cluster)
- **Kustomize** (for Kubernetes manifests; integrated with `kubectl` 1.14+)
- **Go** (to build the binary locally, optional if only using Docker)

---

### **Step 2: Build and Push the Docker Image**

#### **1. Build the Controller Binary**
If you'd like to test locally before creating the Docker image:
```bash
go mod tidy            # Ensure dependencies are up to date
go build -o bin/manager main.go
```

#### **2. Build the Docker Image**
Use the provided `Dockerfile` to build the controller image:
```bash
docker build -t ghcr.io/<your-org>/temporary-rbac-controller:v1.0.0 .
```

#### **3. Push the Docker Image**
Push the image to a container registry (e.g., GitHub Container Registry, Docker Hub):
```bash
docker login ghcr.io   # Authenticate with your registry
docker push ghcr.io/<your-org>/temporary-rbac-controller:v1.0.0
```

Replace `<your-org>` with your organization or username on the container registry.

---

### **Step 3: Deploy the CRD**

1. Apply the CustomResourceDefinition (CRD) manifest:
   ```bash
   kubectl apply -f config/crd/bases/rbac.k8s.io_temporaryrbacs.yaml
   ```

2. Verify that the CRD is installed:
   ```bash
   kubectl get crds
   ```
   Look for `temporaryrbacs.rbac.k8s.io` in the output.

---

### **Step 4: Configure and Deploy the Controller**

1. Update the image in the deployment manifest (`config/manager/manager.yaml`):
   ```yaml
   containers:
     - name: manager
       image: ghcr.io/<your-org>/temporary-rbac-controller:v1.0.0
   ```

2. Deploy the controller using Kustomize:
   ```bash
   kubectl apply -k config/default
   ```

3. Verify the controller deployment:
   ```bash
   kubectl get pods -n kube-system
   ```

   Look for a pod with the name `temporary-rbac-controller`.

---

### **Step 5: Deploy a Sample `TemporaryRBAC` Resource**

1. Apply a sample `TemporaryRBAC` resource:
   ```bash
   kubectl apply -f config/samples/temporaryrbac_v1_temporaryrbac.yaml
   ```

2. Verify that the `TemporaryRBAC` resource is created:
   ```bash
   kubectl get temporaryrbac
   ```

   Look for your sample resource (`example-temporary-rbac`) in the output.

3. Check the associated RoleBinding or ClusterRoleBinding:
   ```bash
   kubectl get rolebindings -n default
   kubectl get clusterrolebindings
   ```

   Ensure the binding matches the subject and role specified in the `TemporaryRBAC` resource.

---

### **Step 6: Monitor Expiration and Cleanup**

1. Check the `TemporaryRBAC` resource status:
   ```bash
   kubectl describe temporaryrbac example-temporary-rbac
   ```

2. Confirm that the RoleBinding or ClusterRoleBinding is deleted automatically after the specified duration:
   ```bash
   kubectl get rolebindings -n default
   ```

---

### **Summary of Commands**

```bash
# Step 1: Build the Docker image
docker build -t ghcr.io/<your-org>/temporary-rbac-controller:v1.0.0 .

# Step 2: Push the image to the registry
docker login ghcr.io
docker push ghcr.io/<your-org>/temporary-rbac-controller:v1.0.0

# Step 3: Deploy the CRD
kubectl apply -f config/crd/bases/rbac.k8s.io_temporaryrbacs.yaml

# Step 4: Deploy the controller
kubectl apply -k config/default

# Step 5: Apply a sample TemporaryRBAC resource
kubectl apply -f config/samples/temporaryrbac_v1_temporaryrbac.yaml

# Step 6: Monitor the controller and TemporaryRBAC behavior
kubectl get pods -n kube-system
kubectl get temporaryrbac
kubectl describe temporaryrbac example-temporary-rbac
```
