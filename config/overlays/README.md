# Kustomize Overlays

This directory contains Kustomize overlays for deploying the Kruize operator to different cluster types with platform-specific configurations.

## Available Overlays

### OpenShift (`openshift/`) - Default

Deploys the Kruize operator to OpenShift clusters with the following configuration:
- **Namespace**: `openshift-tuning`
- **Platform**: OpenShift 4.x+
- **Features**: Includes OpenShift-specific resources like Routes and SecurityContextConstraints

**Usage:**
```bash
# Using kubectl directly
kubectl apply -k config/overlays/openshift

# Using make (default overlay)
make deploy IMG=<your-registry>/kruize-operator:tag

# Using platform-specific make target
make deploy-openshift IMG=<your-registry>/kruize-operator:tag
```

### Local (`local/`)

Deploys the Kruize operator to local Kubernetes clusters (Minikube, KIND, etc.) with the following configuration:
- **Namespace**: `monitoring`
- **Platform**: Kubernetes 1.23.0+
- **Features**: Standard Kubernetes resources with Ingress support

**Usage:**
```bash
# Using kubectl directly
kubectl apply -k config/overlays/local

# Using make with OVERLAY variable
make deploy OVERLAY=local IMG=<your-registry>/kruize-operator:tag

# Using platform-specific make targets
make deploy-minikube IMG=<your-registry>/kruize-operator:tag  # For Minikube
make deploy-kind IMG=<your-registry>/kruize-operator:tag      # For KIND
```

## Make Deploy Commands

The operator provides multiple ways to deploy using make targets:

### Platform-Specific Targets (Recommended)

- **OpenShift**:
  ```bash
  make deploy-openshift IMG=<your-registry>/kruize-operator:tag
  ```
  Deploys to the `openshift-tuning` namespace.

- **Minikube**:
  ```bash
  make deploy-minikube IMG=<your-registry>/kruize-operator:tag
  ```
  Deploys to the `monitoring` namespace.

- **KIND**:
  ```bash
  make deploy-kind IMG=<your-registry>/kruize-operator:tag
  ```
  Deploys to the `monitoring` namespace.

### Using OVERLAY Variable

Alternatively, use the `OVERLAY` variable with `make deploy`:

- **Default (OpenShift)**:
  ```bash
  make deploy IMG=<your-registry>/kruize-operator:tag
  # Equivalent to: make deploy OVERLAY=openshift IMG=<your-registry>/kruize-operator:tag
  ```

- **Local (Minikube/KIND)**:
  ```bash
  make deploy OVERLAY=local IMG=<your-registry>/kruize-operator:tag
  ```

### Undeployment

Similarly, undeploy commands are available:

- **OpenShift**:
  ```bash
  make undeploy-openshift
  ```

- **Minikube**:
  ```bash
  make undeploy-minikube
  ```

- **KIND**:
  ```bash
  make undeploy-kind
  ```

Or using the OVERLAY variable:
```bash
make undeploy OVERLAY=local  # For Minikube/KIND
make undeploy OVERLAY=openshift  # For OpenShift (default)
```

## How Overlays Work

Each overlay references the base configuration in `config/default` and applies platform-specific customizations:

1. **Base Configuration** (`config/default`): Contains the core operator manifests
2. **Overlay Configuration**: Adds or modifies resources for specific platforms
   - Sets the target namespace
   - Applies platform-specific patches
   - Includes additional resources as needed

## Namespace Considerations

The namespace specified in each overlay determines where the operator will be deployed. Ensure that:

1. The namespace exists or will be created during deployment
2. You have appropriate permissions in the target namespace
3. The namespace aligns with your cluster's security policies

## Switching Between Overlays

To switch from one overlay to another:

1. Delete the current deployment:
   ```bash
   kubectl delete -k config/overlays/<current-overlay>
   ```

2. Deploy the new overlay:
   ```bash
   kubectl apply -k config/overlays/<new-overlay>
   ```

## Additional Resources

- [Kustomize Documentation](https://kustomize.io/)
- [Kubernetes Namespaces](https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/)
- [OpenShift Projects](https://docs.openshift.com/container-platform/latest/applications/projects/working-with-projects.html)