package constants

// Default resource values for Kruize components

// Database resource defaults
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

// Kruize application resource defaults
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
	// DefaultKubernetesHostPath is the default host path for Kubernetes PV
	DefaultKubernetesHostPath = "/tmp/data"
)
