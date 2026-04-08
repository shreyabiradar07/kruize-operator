package constants

// Default resource values for Kruize components

// Database resource defaults for OpenShift
// NOTE: For Minikube/Kind clusters, no default resource limits/requests are applied
// for the database unless explicitly specified in the CR.
// This allows for more flexible resource allocation in local development environments.
const (
	// DefaultDBCPURequest is the default CPU request for the database
	DefaultDBCPURequest = "0.5"
	// DefaultDBCPULimit is the default CPU limit for the database
	DefaultDBCPULimit = "0.5"
	// DefaultDBMemoryRequest is the default memory request for the database
	DefaultDBMemoryRequest = "100Mi"
	// DefaultDBMemoryLimit is the default memory limit for the database
	DefaultDBMemoryLimit = "100Mi"
)

// Kruize application resource defaults for OpenShift
// NOTE: For Minikube/Kind clusters, no default resource limits/requests are applied
// for the Kruize application unless explicitly specified in the CR.
// This allows for more flexible resource allocation in local development environments.
const (
	// DefaultKruizeCPURequest is the default CPU request for Kruize application
	DefaultKruizeCPURequest = "0.7"
	// DefaultKruizeCPULimit is the default CPU limit for Kruize application
	DefaultKruizeCPULimit = "0.7"
	// DefaultKruizeMemoryRequest is the default memory request for Kruize application
	DefaultKruizeMemoryRequest = "768Mi"
	// DefaultKruizeMemoryLimit is the default memory limit for Kruize application
	DefaultKruizeMemoryLimit = "768Mi"
)

// OpenShift PV/PVC defaults
const (
	// DefaultOpenShiftPVStorageSize is the default PV storage size for OpenShift
	DefaultOpenShiftPVStorageSize = "500Mi"
	// DefaultOpenShiftStorageClassName is the default storage class for OpenShift
	DefaultOpenShiftStorageClassName = "manual"
	// DefaultOpenShiftHostPath is the default host path for OpenShift PV
	DefaultOpenShiftHostPath = "/mnt/data"
)

// Kubernetes/Minikube/Kind PV/PVC defaults
const (
	// DefaultKubernetesPVStorageSize is the default PV storage size for Kubernetes/Minikube/Kind
	DefaultKubernetesPVStorageSize = "1Gi"
	// DefaultKubernetesStorageClassName is the default storage class for Kubernetes/Minikube/Kind
	// Empty string means no storage class will be used
	DefaultKubernetesStorageClassName = ""
	// DefaultKubernetesHostPath is the default host path for Kubernetes PV
	DefaultKubernetesHostPath = "/data/postgres"
)
