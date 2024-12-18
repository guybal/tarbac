# Load restart_process extension
load('ext://restart_process', 'docker_build_with_restart')

# Paths and configuration
GO_PROJECT_DIR = './'
K8S_MANIFESTS = [
    './config/crd/bases/rbac.k8s.io_temporaryrbacs.yaml',
    './config/crd/bases/rbac.k8s.io_clustertemporaryrbacs.yaml',
    './config/manager/manager.yaml',
    './config/manager/rbac.yaml',
    './config/manager/sa.yaml',
]

local_resource(
  'compile',
  'go mod tidy && ' +
  'CGO_ENABLED=0 GOOS=linux go build -a -o manager main.go',
  deps=['./main.go', './go.mod', './api','./config','./controllers'],
)

# Use docker_build_with_restart for live code updates
docker_build_with_restart(
    'guybalmas/temporary-rbac-controller',
    '.',
    dockerfile='./Dockerfile-tilt',
    entrypoint=['/manager'],
    only=['./bin','./api','./config','./controllers', './go.mod', './go.sum', './main.go'],
    live_update=[
        sync('./bin/manager', '/workspace'),  # Sync local changes to the container
    ],
    restart_file='/tmp/.restart-proc',
)

# Load Kubernetes manifests
k8s_yaml(K8S_MANIFESTS)

# Map the Docker image to the Kubernetes resource
k8s_resource(
    workload='temporary-rbac-controller',
    resource_deps=['compile'],
    port_forwards=8080,  # Forward port 8080 for debugging
    extra_pod_selectors=[{'app.kubernetes.io/name': 'manager'}],  # Adjust label as needed
)

allow_k8s_contexts('aks-tap-build')
