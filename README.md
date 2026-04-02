# kruize-operator

A Kubernetes Operator to automate deployment of [Kruize Autotune](https://github.com/kruize/autotune), a resource optimization tool for Kubernetes workloads.

## Description

The Kruize operator simplifies deployment and management of Kruize on Kubernetes and OpenShift clusters. It provides a declarative way to configure and deploy Kruize components including the core Kruize service and UI through Custom Resource Definitions (CRDs).

For examples of running Kruize and the operator, see [kruize-demos](https://github.com/kruize/kruize-demos/tree/main/monitoring/local_monitoring). You can customize the YAML file (found in config/samples) to deploy with your preferred options.

## SEE ALSO

* [Kruize Autotune](https://github.com/kruize/autotune) - Main Kruize service providing resource optimization recommendations
* [kruize-ui](https://github.com/kruize/kruize-ui) - Web interface for Kruize
* [kruize-demos](https://github.com/kruize/kruize-demos) - Example deployments and demonstrations

## Getting Started

### Prerequisites

**For Deployment:**
- kubectl version v1.26.0+
- Access to a Kubernetes v1.26.0+ or OpenShift v4.13+ cluster
- [Prometheus](https://github.com/prometheus/prometheus) (for Minikube, Kind clusters)

**For Building/Development:**
- Go version v1.21.0+ (tested with v1.22.5 and v1.24.0)
- [operator-sdk](https://github.com/operator-framework/operator-sdk) v1.37.0+ (as specified in Makefile)
- Docker version 17.03+

### Configuration

**Environment Variables:**

The operator supports the following environment variables for customizing default container images:

| Variable | Description |
|----------|-------------|
| `DEFAULT_AUTOTUNE_IMAGE` | Override the default Kruize Autotune container image |
| `DEFAULT_AUTOTUNE_UI_IMAGE` | Override the default Kruize UI container image |


**Example Usage:**
```sh
# Use custom registry/versions
export DEFAULT_AUTOTUNE_IMAGE="my-registry.io/kruize/autotune_operator:custom-tag"
export DEFAULT_AUTOTUNE_UI_IMAGE="my-registry.io/kruize/kruize-ui:custom-tag"
```

These environment variables are checked once at operator startup. When the operator creates Kruize resources with empty `autotune_image` or `autotune_ui_image` fields in the CR spec, it uses these environment variable values (if set) or the built-in defaults. If the CR explicitly specifies image values, those take precedence over environment variables.

### Deployment

The operator uses Kustomize overlays to manage platform-specific configurations:
- **OpenShift** (default): Deploys to `openshift-tuning` namespace
- **Local (Minikube/KIND)**: Deploys to `monitoring` namespace

**Quick Start:**

1. Build and push your image:
   ```sh
   make docker-build docker-push IMG=<some-registry>/kruize-operator:tag
   ```
**NOTE:** Ensure the image is published to a registry accessible from your cluster.

2. Install the CRDs:
   ```sh
   make install
   ```

3. Deploy the operator:

   | Platform | Command | Namespace |
   |----------|---------|-----------|
   | OpenShift | `make deploy-openshift IMG=<registry>/kruize-operator:tag` | `openshift-tuning` |
   | Minikube | `make deploy-minikube IMG=<registry>/kruize-operator:tag` | `monitoring` |
   | KIND | `make deploy-kind IMG=<registry>/kruize-operator:tag` | `monitoring` |

   > **Note**: `IMG` parameter is optional. If not specified, the default image from the Makefile will be used.
   
   > **Alternative**: Use `make deploy OVERLAY=<openshift\|local> IMG=<registry>/kruize-operator:tag`

4. Create a Kruize instance:
   ```sh
   # For OpenShift
   kubectl apply -f config/samples/v1alpha1_kruize.yaml -n openshift-tuning
   
   # For Minikube/KIND
   kubectl apply -f config/samples/v1alpha1_kruize.yaml -n monitoring
   ```
   >**NOTE**: Before applying for Minikube/KIND, update [`config/samples/v1alpha1_kruize.yaml`](config/samples/v1alpha1_kruize.yaml):
   >- Set `cluster_type: "minikube"` or `cluster_type: "kind"`
   >- Set `namespace: "monitoring"` (instead of `"openshift-tuning"`)

**For detailed deployment options, overlay configurations, and advanced usage**, see [config/overlays/README.md](config/overlays/README.md).

**NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin privileges or be logged in as admin.

### To Uninstall

1. Delete the Kruize instance(CR):
   ```sh
   # For OpenShift
   kubectl delete -f config/samples/v1alpha1_kruize.yaml -n openshift-tuning
   
   # For Minikube/KIND
   kubectl delete -f config/samples/v1alpha1_kruize.yaml -n monitoring
   ```

2. Undeploy the controller:
   ```sh
   make undeploy-openshift  # For OpenShift
   make undeploy-minikube   # For Minikube
   make undeploy-kind       # For KIND
   ```

3. Delete the CRDs:
   ```sh
   make uninstall
   ```

For more undeployment options, see [config/overlays/README.md](config/overlays/README.md).

## BUILDING

See [Prerequisites](#prerequisites) section above for required tools and versions.

**Instructions**

`make generate manifests` will trigger code/YAML generation and compile the operator controller manager.

`make docker-build IMG=<registry>/kruize-operator:tag` will build an OCI image. If `IMG` is not specified, the default image from the Makefile will be used.

`make bundle` will create an OLM bundle in the `bundle/` directory. `make bundle-build` will create an OCI image of this bundle.

`make catalog-build` will build an OCI image of the operator catalog.

## DEVELOPMENT

Run the operator locally:
```sh
make run
```
This runs the controller manager as a process on your local machine. Note that it will not have access to certain in-cluster resources.

## TESTING

**Run unit tests:**
```sh
make test
```

**Run end-to-end tests:**

The `test-e2e` target supports optional flags for customizing the test environment:

```sh
# Default (OpenShift cluster)
make test-e2e
```
This requires a Kubernetes or OpenShift cluster. Recommended: Minikube, KIND, or OpenShift.

**For detailed testing documentation**, see:
- [Operator Tests Documentation](test/Operator_tests.md)

## License

Apache License 2.0, see [LICENSE](/LICENSE).

