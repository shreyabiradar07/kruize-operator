/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// KruizeSpec defines the desired state of Kruize
type KruizeSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Type of Kubernetes cluster (openshift, minikube, or kind)
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Cluster Type",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:select:openshift","urn:alm:descriptor:com.tectonic.ui:select:minikube","urn:alm:descriptor:com.tectonic.ui:select:kind"}
	Cluster_type      string `json:"cluster_type"`

	// Container image for Kruize Autotune
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Autotune Image",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text"}
	Autotune_image    string `json:"autotune_image"`

	// Container image for Kruize UI
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Autotune UI Image",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text"}
	Autotune_ui_image string `json:"autotune_ui_image"`

	// Target namespace for Kruize deployment
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Namespace",xDescriptors={"urn:alm:descriptor:com.tectonic.ui:text"}
	Namespace         string `json:"namespace"`

	// Resource configuration for Kruize components
	// +optional
	Resources *ResourceConfig `json:"resources,omitempty"`
}

// ResourceConfig defines resource requirements for Kruize components
type ResourceConfig struct {
	// Database resource configuration
	// +optional
	Database *ContainerResources `json:"database,omitempty"`

	// Kruize application resource configuration
	// +optional
	Kruize *ContainerResources `json:"kruize,omitempty"`

	// Persistent Volume configuration
	// +optional
	PersistentVolume *PersistentVolumeConfig `json:"persistentVolume,omitempty"`
}

// ContainerResources defines CPU and memory resource requirements
type ContainerResources struct {
	// CPU request (e.g., "0.5", "500m")
	// +optional
	CPURequest string `json:"cpuRequest,omitempty"`

	// CPU limit (e.g., "0.5", "500m")
	// +optional
	CPULimit string `json:"cpuLimit,omitempty"`

	// Memory request (e.g., "100Mi", "1Gi")
	// +optional
	MemoryRequest string `json:"memoryRequest,omitempty"`

	// Memory limit (e.g., "100Mi", "1Gi")
	// +optional
	MemoryLimit string `json:"memoryLimit,omitempty"`
}

// PersistentVolumeConfig defines PV and PVC configuration
type PersistentVolumeConfig struct {
	// Storage size for PersistentVolume (e.g., "500Mi", "1Gi")
	// If not specified, defaults to 500Mi for OpenShift, 1Gi for Kubernetes
	// +optional
	PVStorageSize string `json:"pvStorageSize,omitempty"`

	// Storage size for PersistentVolumeClaim (e.g., "500Mi", "1Gi")
	// If not specified, uses PVStorageSize value
	// +optional
	PVCStorageSize string `json:"pvcStorageSize,omitempty"`

	// Storage class name
	// +optional
	StorageClassName string `json:"storageClassName,omitempty"`

	// Host path for the persistent volume
	// +optional
	HostPath string `json:"hostPath,omitempty"`

	// Access modes for the persistent volume
	// Valid values: ReadWriteOnce, ReadOnlyMany, ReadWriteMany, ReadWriteOncePod
	// +optional
	// +kubebuilder:validation:Enum=ReadWriteOnce;ReadOnlyMany;ReadWriteMany;ReadWriteOncePod
	AccessModes []string `json:"accessModes,omitempty"`
}

// KruizeStatus defines the observed state of Kruize
type KruizeStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Nodes []string `json:"nodes"`
}

// Kruize contains configuration options for controlling the deployment of the Kruize
// application and its related components. A Kruize instance must be created to instruct
// the operator to deploy the Kruize application.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +operator-sdk:csv:customresourcedefinitions:resources={{Deployment,v1},{Service,v1},{ServiceAccount,v1},{ConfigMap,v1},{PersistentVolume,v1},{PersistentVolumeClaim,v1},{StorageClass,v1}}
type Kruize struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KruizeSpec   `json:"spec,omitempty"`
	Status KruizeStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// KruizeList contains a list of Kruize
type KruizeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Kruize `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Kruize{}, &KruizeList{})
}
