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

package controller

import (
	"context"
	"os"
	"time"
	"fmt"

	"crypto/tls"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kruizev1alpha1 "github.com/kruize/kruize-operator/api/v1alpha1"
	"github.com/kruize/kruize-operator/internal/constants"
	"github.com/kruize/kruize-operator/internal/utils"
)

// getTestContext returns a context for use in tests
func getTestContext() context.Context {
	return context.Background()
}

// findTypedResource is a helper function to find a specific resource by type, name, and label
func findTypedResource[T client.Object](resources []client.Object, name string, labelKey string, labelValue string) T {
	var zero T
	for _, resource := range resources {
		if typed, ok := resource.(T); ok {
			if typed.GetName() == name {
				if labels := typed.GetLabels(); labels != nil && labels[labelKey] == labelValue {
					return typed
				}
			}
		}
	}
	return zero
}

// findContainerByName is a helper function to find a specific container by name in a container list
func findContainerByName(containers []corev1.Container, name string) *corev1.Container {
	for i := range containers {
		if containers[i].Name == name {
			return &containers[i]
		}
	}
	return nil
}

// Helper function to find a Deployment by name in a list of resources
func findDeployment(resources []client.Object, name string) *appsv1.Deployment {
	for _, resource := range resources {
		if resource.GetObjectKind().GroupVersionKind().Kind == "Deployment" && resource.GetName() == name {
			if deployment, ok := resource.(*appsv1.Deployment); ok {
				return deployment
			}
		}
	}
	return nil
}

// Helper function to find a PersistentVolume in a list of resources
func findPersistentVolume(resources []client.Object) *corev1.PersistentVolume {
	for _, resource := range resources {
		if resource.GetObjectKind().GroupVersionKind().Kind == "PersistentVolume" {
			if pv, ok := resource.(*corev1.PersistentVolume); ok {
				return pv
			}
		}
	}
	return nil
}

// Helper function to find a PersistentVolumeClaim in a list of resources
func findPersistentVolumeClaim(resources []client.Object) *corev1.PersistentVolumeClaim {
	for _, resource := range resources {
		if resource.GetObjectKind().GroupVersionKind().Kind == "PersistentVolumeClaim" {
			if pvc, ok := resource.(*corev1.PersistentVolumeClaim); ok {
				return pvc
			}
		}
	}
	return nil
}

// Helper function to get the first container from a deployment and verify it exists
func getContainer(deployment *appsv1.Deployment, expectedName string) *corev1.Container {
	Expect(deployment).NotTo(BeNil(), "Deployment should exist")
	Expect(deployment.Spec.Template.Spec.Containers).NotTo(BeEmpty(), "Deployment should have containers")
	container := &deployment.Spec.Template.Spec.Containers[0]
	Expect(container.Name).To(Equal(expectedName), "Container name should match")
	return container
}

var _ = Describe("Kruize Controller", func() {
	ctx := context.Background()

	//setting test mode for the controller
	BeforeEach(func() {
		os.Setenv("KRUIZE_TEST_MODE", "true")
	})

	AfterEach(func() {
		os.Unsetenv("KRUIZE_TEST_MODE")
	})
	Context("Test mode behavior", func() {
		var reconciler *KruizeReconciler

		BeforeEach(func() {
			reconciler = &KruizeReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
		})

		It("should detect test mode correctly", func() {
			os.Setenv("KRUIZE_TEST_MODE", "true")
			Expect(reconciler.isTestMode()).To(BeTrue())

			os.Setenv("KRUIZE_TEST_MODE", "false")
			Expect(reconciler.isTestMode()).To(BeFalse())

			os.Unsetenv("KRUIZE_TEST_MODE")
			Expect(reconciler.isTestMode()).To(BeFalse())
		})

		It("should skip pod waiting in test mode", func() {
			os.Setenv("KRUIZE_TEST_MODE", "true")

			// This should return immediately without error
			err := reconciler.waitForKruizePods(ctx, "test-namespace", time.Second*1)
			Expect(err).NotTo(HaveOccurred())
		})
	})
	Context("When reconciling different cluster types", func() {
		DescribeTable("should handle supported cluster types",
			func(clusterType, namespace, testName string) {
				kruize := &kruizev1alpha1.Kruize{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testName,
						Namespace: "default",
					},
					Spec: kruizev1alpha1.KruizeSpec{
						Cluster_type:      clusterType,
						Namespace:         namespace,
						Autotune_image:    constants.GetDefaultAutotuneImage(),
						Autotune_ui_image: constants.GetDefaultUIImage(),
					},
				}
				Expect(k8sClient.Create(ctx, kruize)).To(Succeed())

				defer func() {
					Expect(k8sClient.Delete(ctx, kruize)).To(Succeed())
				}()

				controllerReconciler := &KruizeReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      testName,
						Namespace: "default",
					},
				})
				Expect(err).NotTo(HaveOccurred())
			},
			Entry("OpenShift cluster type", constants.ClusterTypeOpenShift, "openshift-tuning", "test-kruize-openshift"),
			Entry("minikube cluster type", constants.ClusterTypeMinikube, "kruize", "test-kruize-minikube"),
			Entry("kind cluster type", constants.ClusterTypeKind, "kruize", "test-kruize-kind"),
		)

		DescribeTable("should reject invalid cluster types",
			func(clusterType, testName, expectedErrorSubstring string, shouldCheckSupportedTypes bool) {
				kruize := &kruizev1alpha1.Kruize{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testName,
						Namespace: "default",
					},
					Spec: kruizev1alpha1.KruizeSpec{
						Cluster_type:      clusterType,
						Namespace:         "test",
						Autotune_image:    constants.GetDefaultAutotuneImage(),
						Autotune_ui_image: constants.GetDefaultUIImage(),
					},
				}
				Expect(k8sClient.Create(ctx, kruize)).To(Succeed())

				defer func() {
					Expect(k8sClient.Delete(ctx, kruize)).To(Succeed())
				}()

				controllerReconciler := &KruizeReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      testName,
						Namespace: "default",
					},
				})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unsupported cluster type"))
				if expectedErrorSubstring != "" {
					Expect(err.Error()).To(ContainSubstring(expectedErrorSubstring))
				}
				if shouldCheckSupportedTypes {
					Expect(err.Error()).To(ContainSubstring("Supported types are:"))
				}
			},
			Entry("invalid cluster type", "invalid-cluster", "test-kruize-invalid", "invalid-cluster", false),
			Entry("empty cluster type", "", "test-kruize-empty", "", false),
			Entry("unknown cluster type with supported types message", "unknown", "test-kruize-unknown", "", true),
		)

		DescribeTable("should handle case-insensitive cluster types",
			func(clusterType, namespace, testName string) {
				kruize := &kruizev1alpha1.Kruize{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testName,
						Namespace: "default",
					},
					Spec: kruizev1alpha1.KruizeSpec{
						Cluster_type:      clusterType,
						Namespace:         namespace,
						Autotune_image:    constants.GetDefaultAutotuneImage(),
						Autotune_ui_image: constants.GetDefaultUIImage(),
					},
				}
				Expect(k8sClient.Create(ctx, kruize)).To(Succeed())

				defer func() {
					Expect(k8sClient.Delete(ctx, kruize)).To(Succeed())
				}()

				controllerReconciler := &KruizeReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      testName,
						Namespace: "default",
					},
				})
				Expect(err).NotTo(HaveOccurred())
			},
			Entry("OpenShift with capital O", "OpenShift", "openshift-tuning", "test-kruize-openshift-capital"),
			Entry("MINIKUBE all caps", "MINIKUBE", "kruize", "test-kruize-minikube-caps"),
			Entry("Kind with capital K", "Kind", "kruize", "test-kruize-kind-capital"),
			Entry("MixedCase minikube", "MiniKube", "kruize", "test-kruize-minikube-mixed"),
		)

		DescribeTable("should not create resources for unsupported cluster types",
			func(clusterType, testName string) {
				testNamespace := "test-" + clusterType + "-namespace"
				kruize := &kruizev1alpha1.Kruize{
					ObjectMeta: metav1.ObjectMeta{
						Name:      testName,
						Namespace: "default",
					},
					Spec: kruizev1alpha1.KruizeSpec{
						Cluster_type:      clusterType,
						Namespace:         testNamespace,
						Autotune_image:    constants.GetDefaultAutotuneImage(),
						Autotune_ui_image: constants.GetDefaultUIImage(),
					},
				}
				Expect(k8sClient.Create(ctx, kruize)).To(Succeed())

				defer func() {
					Expect(k8sClient.Delete(ctx, kruize)).To(Succeed())
				}()

				controllerReconciler := &KruizeReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				// Attempt reconciliation - should fail with validation error
				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      testName,
						Namespace: "default",
					},
				})

				// Verify error is returned
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unsupported cluster type"))
				Expect(err.Error()).To(ContainSubstring(clusterType))

				// Verify no namespace was created for Kruize components
				namespaceList := &corev1.NamespaceList{}
				err = k8sClient.List(ctx, namespaceList)
				Expect(err).NotTo(HaveOccurred())

				namespaceExists := false
				for _, ns := range namespaceList.Items {
					if ns.Name == testNamespace {
						namespaceExists = true
						break
					}
				}
				Expect(namespaceExists).To(BeFalse(), "Namespace should not be created for invalid cluster type")

				// Verify no deployments were created in the test namespace
				deploymentList := &appsv1.DeploymentList{}
				err = k8sClient.List(ctx, deploymentList, client.InNamespace(testNamespace))
				if err == nil {
					Expect(deploymentList.Items).To(BeEmpty(), "No deployments should be created for invalid cluster type")
				}

				// Verify no services were created in the test namespace
				serviceList := &corev1.ServiceList{}
				err = k8sClient.List(ctx, serviceList, client.InNamespace(testNamespace))
				if err == nil {
					Expect(serviceList.Items).To(BeEmpty(), "No services should be created for invalid cluster type")
				}
			},
			Entry("gke cluster type", "gke", "test-kruize-gke"),
			Entry("eks cluster type", "eks", "test-kruize-eks"),
		)
	})

	Context("Resource generation", func() {
		It("should generate cluster-scoped resources for OpenShift", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", constants.ClusterTypeOpenShift, nil, getTestContext())

			clusterResources := generator.ClusterScopedResources()
			Expect(clusterResources).NotTo(BeEmpty())
			Expect(len(clusterResources)).To(BeNumerically(">", 0))
		})

		It("should generate namespaced resources for OpenShift", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", constants.ClusterTypeOpenShift, nil, getTestContext())

			namespacedResources := generator.NamespacedResources()
			Expect(namespacedResources).NotTo(BeEmpty())
			Expect(len(namespacedResources)).To(BeNumerically(">", 0))
		})

		It("should generate Kubernetes cluster-scoped resources", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", constants.ClusterTypeMinikube, nil, getTestContext())

			clusterResources := generator.KubernetesClusterScopedResources()
			Expect(clusterResources).NotTo(BeEmpty())
			Expect(len(clusterResources)).To(BeNumerically(">", 0))
		})

		It("should generate Kubernetes namespaced resources", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", constants.ClusterTypeMinikube, nil, getTestContext())

			namespacedResources := generator.KubernetesNamespacedResources()
			Expect(namespacedResources).NotTo(BeEmpty())
			Expect(len(namespacedResources)).To(BeNumerically(">", 0))
		})

		It("should use default images when not specified", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", constants.ClusterTypeOpenShift, nil, getTestContext())

			Expect(generator.Autotune_image).To(Equal(constants.GetDefaultAutotuneImage()))
			Expect(generator.Autotune_ui_image).To(Equal(constants.GetDefaultUIImage()))
		})

		It("should use custom images when specified", func() {
			customImage := "custom/image:v1.0"
			customUIImage := "custom/ui:v1.0"
			generator := utils.NewKruizeResourceGenerator("test-namespace", customImage, customUIImage, constants.ClusterTypeOpenShift, nil, getTestContext())

			Expect(generator.Autotune_image).To(Equal(customImage))
			Expect(generator.Autotune_ui_image).To(Equal(customUIImage))
		})

		It("should apply custom ResourceConfig to Kruize and Database deployments", func() {
			cpuRequest := "200m"
			cpuLimit := "500m"
			memRequest := "256Mi"
			memLimit := "512Mi"

			kruize := &kruizev1alpha1.Kruize{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kruize",
					Namespace: "test-namespace",
				},
				Spec: kruizev1alpha1.KruizeSpec{
					Resources: &kruizev1alpha1.ResourceConfig{
						Database: &kruizev1alpha1.ContainerResources{
							CPURequest:    cpuRequest,
							CPULimit:      cpuLimit,
							MemoryRequest: memRequest,
							MemoryLimit:   memLimit,
						},
						Kruize: &kruizev1alpha1.ContainerResources{
							CPURequest:    cpuRequest,
							CPULimit:      cpuLimit,
							MemoryRequest: memRequest,
							MemoryLimit:   memLimit,
						},
					},
				},
			}

			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", constants.ClusterTypeOpenShift, kruize.Spec.Resources, getTestContext())

			namespacedResources := generator.NamespacedResources()
			Expect(namespacedResources).NotTo(BeEmpty())

			var kruizeDeployment *appsv1.Deployment
			var dbDeployment *appsv1.Deployment

			for _, obj := range namespacedResources {
				deploy, ok := obj.(*appsv1.Deployment)
				if !ok {
					continue
				}
				switch deploy.Name {
				case "kruize":
					kruizeDeployment = deploy
				case "kruize-db-deployment":
					dbDeployment = deploy
				}
			}

			Expect(kruizeDeployment).NotTo(BeNil(), "expected Kruize deployment to be generated")
			Expect(dbDeployment).NotTo(BeNil(), "expected Database deployment to be generated")

			// Verify Kruize container resources
			kruizeContainer := findContainerByName(kruizeDeployment.Spec.Template.Spec.Containers, "kruize")
			Expect(kruizeContainer).NotTo(BeNil(), "expected to find kruize container")
			Expect(kruizeContainer.Resources.Requests.Cpu().String()).To(Equal(cpuRequest))
			Expect(kruizeContainer.Resources.Limits.Cpu().String()).To(Equal(cpuLimit))
			Expect(kruizeContainer.Resources.Requests.Memory().String()).To(Equal(memRequest))
			Expect(kruizeContainer.Resources.Limits.Memory().String()).To(Equal(memLimit))

			// Verify Database container resources
			dbContainer := findContainerByName(dbDeployment.Spec.Template.Spec.Containers, "kruize-db")
			Expect(dbContainer).NotTo(BeNil(), "expected to find kruize-db container")
			Expect(dbContainer.Resources.Requests.Cpu().String()).To(Equal(cpuRequest))
			Expect(dbContainer.Resources.Limits.Cpu().String()).To(Equal(cpuLimit))
			Expect(dbContainer.Resources.Requests.Memory().String()).To(Equal(memRequest))
			Expect(dbContainer.Resources.Limits.Memory().String()).To(Equal(memLimit))
		})

		It("should allow partial resource overrides while preserving defaults", func() {
			// First, generate resources with default configuration to capture default resources
			defaultGenerator := utils.NewKruizeResourceGenerator("test-namespace", "", "", constants.ClusterTypeOpenShift, nil, getTestContext())
			defaultNamespacedResources := defaultGenerator.NamespacedResources()

			var defaultKruizeDeployment *appsv1.Deployment
			var defaultDBDeployment *appsv1.Deployment

			for _, obj := range defaultNamespacedResources {
				deploy, ok := obj.(*appsv1.Deployment)
				if !ok {
					continue
				}
				switch deploy.Name {
				case "kruize":
					defaultKruizeDeployment = deploy
				case "kruize-db-deployment":
					defaultDBDeployment = deploy
				}
			}

			Expect(defaultKruizeDeployment).NotTo(BeNil(), "default Kruize deployment should exist")
			Expect(defaultDBDeployment).NotTo(BeNil(), "default DB deployment should exist")

			defaultKruizeContainer := findContainerByName(defaultKruizeDeployment.Spec.Template.Spec.Containers, "kruize")
			defaultDBContainer := findContainerByName(defaultDBDeployment.Spec.Template.Spec.Containers, "kruize-db")
			Expect(defaultKruizeContainer).NotTo(BeNil(), "default kruize container should exist")
			Expect(defaultDBContainer).NotTo(BeNil(), "default kruize-db container should exist")

			// Now, create a ResourceConfig that only overrides the CPU request for both components
			partialConfig := &kruizev1alpha1.ResourceConfig{
				Database: &kruizev1alpha1.ContainerResources{
					CPURequest: "250m",
					// CPULimit, MemoryRequest, and MemoryLimit are intentionally omitted
				},
				Kruize: &kruizev1alpha1.ContainerResources{
					CPURequest: "300m",
					// CPULimit, MemoryRequest, and MemoryLimit are intentionally omitted
				},
			}

			partialGenerator := utils.NewKruizeResourceGenerator("test-namespace", "", "", constants.ClusterTypeOpenShift, partialConfig, getTestContext())
			partialNamespacedResources := partialGenerator.NamespacedResources()

			var partialKruizeDeployment *appsv1.Deployment
			var partialDBDeployment *appsv1.Deployment

			for _, obj := range partialNamespacedResources {
				deploy, ok := obj.(*appsv1.Deployment)
				if !ok {
					continue
				}
				switch deploy.Name {
				case "kruize":
					partialKruizeDeployment = deploy
				case "kruize-db-deployment":
					partialDBDeployment = deploy
				}
			}

			Expect(partialKruizeDeployment).NotTo(BeNil(), "partial Kruize deployment should exist")
			Expect(partialDBDeployment).NotTo(BeNil(), "partial DB deployment should exist")

			partialKruizeContainer := findContainerByName(partialKruizeDeployment.Spec.Template.Spec.Containers, "kruize")
			partialDBContainer := findContainerByName(partialDBDeployment.Spec.Template.Spec.Containers, "kruize-db")
			Expect(partialKruizeContainer).NotTo(BeNil(), "partial kruize container should exist")
			Expect(partialDBContainer).NotTo(BeNil(), "partial kruize-db container should exist")

			// Verify that the CPU request was overridden for DB
			Expect(partialDBContainer.Resources.Requests.Cpu().String()).To(Equal("250m"))
			// Verify that CPU limit and memory request/limit are still using the defaults for DB
			Expect(partialDBContainer.Resources.Limits.Cpu().String()).To(Equal(defaultDBContainer.Resources.Limits.Cpu().String()))
			Expect(partialDBContainer.Resources.Requests.Memory().String()).To(Equal(defaultDBContainer.Resources.Requests.Memory().String()))
			Expect(partialDBContainer.Resources.Limits.Memory().String()).To(Equal(defaultDBContainer.Resources.Limits.Memory().String()))

			// Verify that the CPU request was overridden for Kruize
			Expect(partialKruizeContainer.Resources.Requests.Cpu().String()).To(Equal("300m"))
			// Verify that CPU limit and memory request/limit are still using the defaults for Kruize
			Expect(partialKruizeContainer.Resources.Limits.Cpu().String()).To(Equal(defaultKruizeContainer.Resources.Limits.Cpu().String()))
			Expect(partialKruizeContainer.Resources.Requests.Memory().String()).To(Equal(defaultKruizeContainer.Resources.Requests.Memory().String()))
			Expect(partialKruizeContainer.Resources.Limits.Memory().String()).To(Equal(defaultKruizeContainer.Resources.Limits.Memory().String()))
		})
	})

	Context("RBAC and ConfigMap manifest generation", func() {
		It("should generate RBAC manifests correctly for OpenShift", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", constants.ClusterTypeOpenShift, nil, getTestContext())

			clusterResources := generator.ClusterScopedResources()

			// Check that RBAC resources are present
			var hasClusterRole, hasClusterRoleBinding bool
			for _, resource := range clusterResources {
				kind := resource.GetObjectKind().GroupVersionKind().Kind
				if kind == "ClusterRole" {
					hasClusterRole = true
				}
				if kind == "ClusterRoleBinding" {
					hasClusterRoleBinding = true
				}
			}

			Expect(hasClusterRole).To(BeTrue(), "ClusterRole should be generated")
			Expect(hasClusterRoleBinding).To(BeTrue(), "ClusterRoleBinding should be generated")
		})

		It("should generate RBAC manifests correctly for Kubernetes", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", constants.ClusterTypeMinikube, nil, getTestContext())

			clusterResources := generator.KubernetesClusterScopedResources()

			// Check that RBAC resources are present
			var hasClusterRole, hasClusterRoleBinding bool
			for _, resource := range clusterResources {
				kind := resource.GetObjectKind().GroupVersionKind().Kind
				if kind == "ClusterRole" {
					hasClusterRole = true
				}
				if kind == "ClusterRoleBinding" {
					hasClusterRoleBinding = true
				}
			}

			Expect(hasClusterRole).To(BeTrue(), "ClusterRole should be generated")
			Expect(hasClusterRoleBinding).To(BeTrue(), "ClusterRoleBinding should be generated")
		})

		It("should generate ConfigMap correctly for OpenShift", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", constants.ClusterTypeOpenShift, nil, getTestContext())

			configMap := generator.KruizeConfigMap()
			Expect(configMap).NotTo(BeNil())
			Expect(configMap.GetName()).To(Equal("kruizeconfig"))
			Expect(configMap.GetNamespace()).To(Equal("test-namespace"))
			Expect(configMap.Data).NotTo(BeEmpty())
		})

		It("should generate ConfigMap correctly for Kubernetes", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", constants.ClusterTypeMinikube, nil, getTestContext())

			configMap := generator.KruizeConfigMapKubernetes()
			Expect(configMap).NotTo(BeNil())
			Expect(configMap.GetName()).To(Equal("kruizeconfig"))
			Expect(configMap.GetNamespace()).To(Equal("test-namespace"))
			Expect(configMap.Data).NotTo(BeEmpty())
		})
	})

	Context("Data source configuration validation", func() {
		It("should have valid data source configuration in ConfigMap for OpenShift", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", constants.ClusterTypeOpenShift, nil, getTestContext())

			configMap := generator.KruizeConfigMap()
			Expect(configMap.Data).To(HaveKey("kruizeconfigjson"))

			// Verify the config contains expected data source fields
			configData := configMap.Data["kruizeconfigjson"]
			Expect(configData).To(ContainSubstring("datasource"))
		})

		It("should have valid data source configuration in ConfigMap for Kubernetes", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", constants.ClusterTypeMinikube, nil, getTestContext())

			configMap := generator.KruizeConfigMapKubernetes()
			Expect(configMap.Data).To(HaveKey("kruizeconfigjson"))

			// Verify the config contains expected data source fields
			configData := configMap.Data["kruizeconfigjson"]
			Expect(configData).To(ContainSubstring("datasource"))
		})
	})

	Context("Kruize deployment manifest generation", func() {
		DescribeTable("should generate valid Kruize deployment manifest with default resources",
			func(clusterType string, resourceMethod func(*utils.KruizeResourceGenerator) []client.Object) {
				generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", clusterType, nil, getTestContext())
				namespacedResources := resourceMethod(generator)

				// Check for Deployment resources and validate default resource configuration
				var kruizeDeployment *appsv1.Deployment
				var kruizeDBDeployment *appsv1.Deployment
				
				for _, resource := range namespacedResources {
					kind := resource.GetObjectKind().GroupVersionKind().Kind
					name := resource.GetName()

					if kind == "Deployment" && name == "kruize" {
						var ok bool
						kruizeDeployment, ok = resource.(*appsv1.Deployment)
						Expect(ok).To(BeTrue(), "Resource should be a valid Deployment")
					}
					if kind == "Deployment" && name == "kruize-db-deployment" {
						var ok bool
						kruizeDBDeployment, ok = resource.(*appsv1.Deployment)
						Expect(ok).To(BeTrue(), "Resource should be a valid Deployment")
					}
				}

				Expect(kruizeDeployment).NotTo(BeNil(), "Kruize deployment should be generated")
				Expect(kruizeDBDeployment).NotTo(BeNil(), "Kruize DB deployment should be generated")

				// Validate Kruize deployment has default resource configuration
				Expect(kruizeDeployment.Spec.Template.Spec.Containers).NotTo(BeEmpty())
				kruizeContainer := kruizeDeployment.Spec.Template.Spec.Containers[0]
				Expect(kruizeContainer.Name).To(Equal("kruize"))
				Expect(kruizeContainer.Resources.Requests.Cpu().String()).To(Equal("700m"))
				Expect(kruizeContainer.Resources.Requests.Memory().String()).To(Equal("768Mi"))
				Expect(kruizeContainer.Resources.Limits.Cpu().String()).To(Equal("700m"))
				Expect(kruizeContainer.Resources.Limits.Memory().String()).To(Equal("768Mi"))

				// Validate DB deployment has default resource configuration
				Expect(kruizeDBDeployment.Spec.Template.Spec.Containers).NotTo(BeEmpty())
				dbContainer := kruizeDBDeployment.Spec.Template.Spec.Containers[0]
				Expect(dbContainer.Name).To(Equal("kruize-db"))
				Expect(dbContainer.Resources.Requests.Cpu().String()).To(Equal("500m"))
				Expect(dbContainer.Resources.Requests.Memory().String()).To(Equal("100Mi"))
				Expect(dbContainer.Resources.Limits.Cpu().String()).To(Equal("500m"))
				Expect(dbContainer.Resources.Limits.Memory().String()).To(Equal("100Mi"))
			},
			Entry("for OpenShift", constants.ClusterTypeOpenShift, func(g *utils.KruizeResourceGenerator) []client.Object {
				return g.NamespacedResources()
			}),
			Entry("for Kubernetes", constants.ClusterTypeMinikube, func(g *utils.KruizeResourceGenerator) []client.Object {
				return g.KubernetesNamespacedResources()
			}),
		)

		DescribeTable("should generate valid Kruize deployment manifest with custom resources",
			func(clusterType string, resourceMethod func(*utils.KruizeResourceGenerator) []client.Object) {
				customResources := &kruizev1alpha1.ResourceConfig{
					Kruize: &kruizev1alpha1.ContainerResources{
						CPURequest:    "2.0",
						CPULimit:      "2.5",
						MemoryRequest: "1Gi",
						MemoryLimit:   "1.5Gi",
					},
					Database: &kruizev1alpha1.ContainerResources{
						CPURequest:    "0.75",
						CPULimit:      "1.5",
						MemoryRequest: "512Mi",
						MemoryLimit:   "1Gi",
					},
				}
				generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", clusterType, customResources, getTestContext())
				namespacedResources := resourceMethod(generator)

				// Check for Deployment resources and validate custom resource configuration
				var kruizeDeployment *appsv1.Deployment
				var kruizeDBDeployment *appsv1.Deployment
				
				for _, resource := range namespacedResources {
					kind := resource.GetObjectKind().GroupVersionKind().Kind
					name := resource.GetName()

					if kind == "Deployment" && name == "kruize" {
						var ok bool
						kruizeDeployment, ok = resource.(*appsv1.Deployment)
						Expect(ok).To(BeTrue(), "Resource should be a valid Deployment")
					}
					if kind == "Deployment" && name == "kruize-db-deployment" {
						var ok bool
						kruizeDBDeployment, ok = resource.(*appsv1.Deployment)
						Expect(ok).To(BeTrue(), "Resource should be a valid Deployment")
					}
				}

				Expect(kruizeDeployment).NotTo(BeNil(), "Kruize deployment should be generated")
				Expect(kruizeDBDeployment).NotTo(BeNil(), "Kruize DB deployment should be generated")

				// Validate Kruize deployment has custom resource configuration
				Expect(kruizeDeployment.Spec.Template.Spec.Containers).NotTo(BeEmpty())
				kruizeContainer := kruizeDeployment.Spec.Template.Spec.Containers[0]
				Expect(kruizeContainer.Name).To(Equal("kruize"))
				Expect(kruizeContainer.Resources.Requests.Cpu().String()).To(Equal("2"))
				Expect(kruizeContainer.Resources.Requests.Memory().String()).To(Equal("1Gi"))
				Expect(kruizeContainer.Resources.Limits.Cpu().String()).To(Equal("2500m"))
				Expect(kruizeContainer.Resources.Limits.Memory().String()).To(Equal("1536Mi"))

				// Validate DB deployment has custom resource configuration
				Expect(kruizeDBDeployment.Spec.Template.Spec.Containers).NotTo(BeEmpty())
				dbContainer := kruizeDBDeployment.Spec.Template.Spec.Containers[0]
				Expect(dbContainer.Name).To(Equal("kruize-db"))
				Expect(dbContainer.Resources.Requests.Cpu().String()).To(Equal("750m"))
				Expect(dbContainer.Resources.Requests.Memory().String()).To(Equal("512Mi"))
				Expect(dbContainer.Resources.Limits.Cpu().String()).To(Equal("1500m"))
				Expect(dbContainer.Resources.Limits.Memory().String()).To(Equal("1Gi"))
			},
			Entry("for OpenShift", constants.ClusterTypeOpenShift, func(g *utils.KruizeResourceGenerator) []client.Object {
				return g.NamespacedResources()
			}),
			Entry("for Kubernetes", constants.ClusterTypeMinikube, func(g *utils.KruizeResourceGenerator) []client.Object {
				return g.KubernetesNamespacedResources()
			}),
		)
	})

	Context("Pod creation validation", func() {
		It("should generate Kruize pod specification", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", constants.ClusterTypeOpenShift, nil, getTestContext())

			namespacedResources := generator.NamespacedResources()

			// Find the Kruize deployment
			kruizeDeployment := findDeployment(namespacedResources, "kruize")
			Expect(kruizeDeployment).NotTo(BeNil(), "Kruize deployment should exist")
			Expect(kruizeDeployment.Spec.Template.Spec.Containers).NotTo(BeEmpty())
			Expect(kruizeDeployment.Spec.Template.Spec.Containers[0].Name).To(Equal("kruize"))
		})

		It("should generate Kruize-ui pod specification", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", constants.ClusterTypeOpenShift, nil, getTestContext())

			namespacedResources := generator.NamespacedResources()

			// Find the Kruize UI pod
			var kruizeUIPod *corev1.Pod
			for _, resource := range namespacedResources {
				if resource.GetObjectKind().GroupVersionKind().Kind == "Pod" && resource.GetName() == "kruize-ui-nginx-pod" {
					var ok bool
					kruizeUIPod, ok = resource.(*corev1.Pod)
					Expect(ok).To(BeTrue(), "Resource should be a valid Pod")
					break
				}
			}

			Expect(kruizeUIPod).NotTo(BeNil(), "Kruize UI pod should exist")
			Expect(kruizeUIPod.Spec.Containers).NotTo(BeEmpty())
		})

		It("should generate Kruize-db pod specification", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", constants.ClusterTypeOpenShift, nil, getTestContext())

			namespacedResources := generator.NamespacedResources()

			// Find the Kruize DB deployment
			kruizeDBDeployment := findDeployment(namespacedResources, "kruize-db-deployment")
			Expect(kruizeDBDeployment).NotTo(BeNil(), "Kruize DB deployment should exist")
			Expect(kruizeDBDeployment.Spec.Template.Spec.Containers).NotTo(BeEmpty())
			Expect(kruizeDBDeployment.Spec.Template.Spec.Containers[0].Name).To(Equal("kruize-db"))
		})

		It("should apply custom resource configuration to Kruize deployment", func() {
			customResources := &kruizev1alpha1.ResourceConfig{
				Kruize: &kruizev1alpha1.ContainerResources{
					CPURequest:    "1.0",
					CPULimit:      "2.0",
					MemoryRequest: "1Gi",
					MemoryLimit:   "2Gi",
				},
			}
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", constants.ClusterTypeOpenShift, customResources, getTestContext())

			namespacedResources := generator.NamespacedResources()

			// Find the Kruize deployment
			kruizeDeployment := findDeployment(namespacedResources, "kruize")
			Expect(kruizeDeployment).NotTo(BeNil(), "Kruize deployment should exist")
			Expect(kruizeDeployment.Spec.Template.Spec.Containers).NotTo(BeEmpty())
			
			container := kruizeDeployment.Spec.Template.Spec.Containers[0]
			Expect(container.Name).To(Equal("kruize"))
			
			// Verify custom resource requests
			Expect(container.Resources.Requests.Cpu().String()).To(Equal("1"))
			Expect(container.Resources.Requests.Memory().String()).To(Equal("1Gi"))
			
			// Verify custom resource limits
			Expect(container.Resources.Limits.Cpu().String()).To(Equal("2"))
			Expect(container.Resources.Limits.Memory().String()).To(Equal("2Gi"))
		})

		It("should apply custom resource configuration to Database deployment", func() {
			customResources := &kruizev1alpha1.ResourceConfig{
				Database: &kruizev1alpha1.ContainerResources{
					CPURequest:    "0.25",
					CPULimit:      "1.0",
					MemoryRequest: "256Mi",
					MemoryLimit:   "512Mi",
				},
			}
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", constants.ClusterTypeOpenShift, customResources, getTestContext())

			namespacedResources := generator.NamespacedResources()

			// Find the Kruize DB deployment
			kruizeDBDeployment := findDeployment(namespacedResources, "kruize-db-deployment")
			Expect(kruizeDBDeployment).NotTo(BeNil(), "Kruize DB deployment should exist")
			Expect(kruizeDBDeployment.Spec.Template.Spec.Containers).NotTo(BeEmpty())
			
			container := kruizeDBDeployment.Spec.Template.Spec.Containers[0]
			Expect(container.Name).To(Equal("kruize-db"))
			
			// Verify custom resource requests
			Expect(container.Resources.Requests.Cpu().String()).To(Equal("250m"))
			Expect(container.Resources.Requests.Memory().String()).To(Equal("256Mi"))
			
			// Verify custom resource limits
			Expect(container.Resources.Limits.Cpu().String()).To(Equal("1"))
			Expect(container.Resources.Limits.Memory().String()).To(Equal("512Mi"))
		})

		It("should use default resources when ResourceConfig is nil", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", constants.ClusterTypeOpenShift, nil, getTestContext())

			namespacedResources := generator.NamespacedResources()

			// Find the Kruize deployment
			kruizeDeployment := findDeployment(namespacedResources, "kruize")
			Expect(kruizeDeployment).NotTo(BeNil(), "Kruize deployment should exist")
			container := kruizeDeployment.Spec.Template.Spec.Containers[0]
			
			// Verify default resource requests
			Expect(container.Resources.Requests.Cpu().String()).To(Equal("700m"))
			Expect(container.Resources.Requests.Memory().String()).To(Equal("768Mi"))
			
			// Verify default resource limits
			Expect(container.Resources.Limits.Cpu().String()).To(Equal("700m"))
			Expect(container.Resources.Limits.Memory().String()).To(Equal("768Mi"))
		})

		It("should apply partial custom resource configuration with defaults", func() {
			customResources := &kruizev1alpha1.ResourceConfig{
				Kruize: &kruizev1alpha1.ContainerResources{
					CPURequest:  "1.5",
					MemoryLimit: "3Gi",
					// CPULimit and MemoryRequest not specified, should use defaults
				},
			}
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", constants.ClusterTypeOpenShift, customResources, getTestContext())

			namespacedResources := generator.NamespacedResources()

			// Find the Kruize deployment
			kruizeDeployment := findDeployment(namespacedResources, "kruize")
			Expect(kruizeDeployment).NotTo(BeNil(), "Kruize deployment should exist")
			container := kruizeDeployment.Spec.Template.Spec.Containers[0]
			
			// Verify custom CPU request
			Expect(container.Resources.Requests.Cpu().String()).To(Equal("1500m"))
			// Verify default memory request
			Expect(container.Resources.Requests.Memory().String()).To(Equal("768Mi"))
			
			// Verify default CPU limit
			Expect(container.Resources.Limits.Cpu().String()).To(Equal("700m"))
			// Verify custom memory limit
			Expect(container.Resources.Limits.Memory().String()).To(Equal("3Gi"))
		})
	})

	Context("Partial ResourceConfig edge cases", func() {
		It("should apply partial Database ResourceConfig with only CPURequest set", func() {
			customResources := &kruizev1alpha1.ResourceConfig{
				Database: &kruizev1alpha1.ContainerResources{
					CPURequest: "0.3",
					// CPULimit, MemoryRequest, and MemoryLimit intentionally omitted
				},
			}
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", constants.ClusterTypeOpenShift, customResources, getTestContext())

			namespacedResources := generator.NamespacedResources()

			// Find the Kruize DB deployment
			kruizeDBDeployment := findDeployment(namespacedResources, "kruize-db-deployment")
			Expect(kruizeDBDeployment).NotTo(BeNil(), "Kruize DB deployment should exist")
			Expect(kruizeDBDeployment.Spec.Template.Spec.Containers).NotTo(BeEmpty())
			
			container := kruizeDBDeployment.Spec.Template.Spec.Containers[0]
			Expect(container.Name).To(Equal("kruize-db"))
			
			// Verify custom CPU request
			Expect(container.Resources.Requests.Cpu().String()).To(Equal("300m"))
			// Verify default values for unset fields
			Expect(container.Resources.Requests.Memory().String()).To(Equal("100Mi"))
			Expect(container.Resources.Limits.Cpu().String()).To(Equal("500m"))
			Expect(container.Resources.Limits.Memory().String()).To(Equal("100Mi"))
		})

		It("should apply partial Kruize ResourceConfig with only MemoryLimit set", func() {
			customResources := &kruizev1alpha1.ResourceConfig{
				Kruize: &kruizev1alpha1.ContainerResources{
					MemoryLimit: "2Gi",
					// CPURequest, CPULimit, and MemoryRequest intentionally omitted
				},
			}
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", constants.ClusterTypeMinikube, customResources, getTestContext())

			namespacedResources := generator.KubernetesNamespacedResources()

			// Find the Kruize deployment
			var kruizeDeployment *appsv1.Deployment
			for _, resource := range namespacedResources {
				if resource.GetObjectKind().GroupVersionKind().Kind == "Deployment" && resource.GetName() == "kruize" {
					var ok bool
					kruizeDeployment, ok = resource.(*appsv1.Deployment)
					Expect(ok).To(BeTrue(), "Resource should be a valid Deployment")
					break
				}
			}

			Expect(kruizeDeployment).NotTo(BeNil(), "Kruize deployment should exist")
			Expect(kruizeDeployment.Spec.Template.Spec.Containers).NotTo(BeEmpty())
			
			container := kruizeDeployment.Spec.Template.Spec.Containers[0]
			Expect(container.Name).To(Equal("kruize"))
			
			// Verify default values for unset fields
			Expect(container.Resources.Requests.Cpu().String()).To(Equal("700m"))
			Expect(container.Resources.Requests.Memory().String()).To(Equal("768Mi"))
			Expect(container.Resources.Limits.Cpu().String()).To(Equal("700m"))
			// Verify custom memory limit
			Expect(container.Resources.Limits.Memory().String()).To(Equal("2Gi"))
		})

		It("should apply partial Database ResourceConfig with mixed custom and default values", func() {
			customResources := &kruizev1alpha1.ResourceConfig{
				Database: &kruizev1alpha1.ContainerResources{
					CPULimit:      "1.0",
					MemoryRequest: "256Mi",
					// CPURequest and MemoryLimit intentionally omitted
				},
			}
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", constants.ClusterTypeMinikube, customResources, getTestContext())

			namespacedResources := generator.KubernetesNamespacedResources()

			// Find the Kruize DB deployment
			var kruizeDBDeployment *appsv1.Deployment
			for _, resource := range namespacedResources {
				if resource.GetObjectKind().GroupVersionKind().Kind == "Deployment" && resource.GetName() == "kruize-db-deployment" {
					var ok bool
					kruizeDBDeployment, ok = resource.(*appsv1.Deployment)
					Expect(ok).To(BeTrue(), "Resource should be a valid Deployment")
					break
				}
			}

			Expect(kruizeDBDeployment).NotTo(BeNil(), "Kruize DB deployment should exist")
			Expect(kruizeDBDeployment.Spec.Template.Spec.Containers).NotTo(BeEmpty())
			
			container := kruizeDBDeployment.Spec.Template.Spec.Containers[0]
			Expect(container.Name).To(Equal("kruize-db"))
			
			// Verify default CPU request
			Expect(container.Resources.Requests.Cpu().String()).To(Equal("500m"))
			// Verify custom memory request
			Expect(container.Resources.Requests.Memory().String()).To(Equal("256Mi"))
			// Verify custom CPU limit
			Expect(container.Resources.Limits.Cpu().String()).To(Equal("1"))
			// Verify default memory limit
			Expect(container.Resources.Limits.Memory().String()).To(Equal("100Mi"))
		})
	})

	Context("PersistentVolume and PVC configuration", func() {
		It("should apply custom PV/PVC configuration for OpenShift", func() {
			customResources := &kruizev1alpha1.ResourceConfig{
				PersistentVolume: &kruizev1alpha1.PersistentVolumeConfig{
					PVStorageSize:    "2Gi",
					PVCStorageSize:   "1Gi",
					StorageClassName: "custom-storage",
					HostPath:         "/custom/path",
					AccessModes:      []string{"ReadWriteOnce"},
				},
			}
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", constants.ClusterTypeOpenShift, customResources, getTestContext())

			clusterResources := generator.ClusterScopedResources()

			// Find the PV
			pv := findPersistentVolume(clusterResources)
			Expect(pv).NotTo(BeNil(), "PersistentVolume should exist")
			Expect(pv.Spec.Capacity.Storage().String()).To(Equal("2Gi"))
			Expect(pv.Spec.StorageClassName).To(Equal("custom-storage"))
			Expect(pv.Spec.PersistentVolumeSource.HostPath.Path).To(Equal("/custom/path"))
			Expect(pv.Spec.AccessModes).To(Equal([]corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}))

			// Find the PVC
			pvc := findPersistentVolumeClaim(clusterResources)
			Expect(pvc).NotTo(BeNil(), "PersistentVolumeClaim should exist")
			Expect(pvc.Spec.Resources.Requests.Storage().String()).To(Equal("1Gi"))
			Expect(*pvc.Spec.StorageClassName).To(Equal("custom-storage"))
			Expect(pvc.Spec.AccessModes).To(Equal([]corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}))
		})

		It("should apply custom PV/PVC configuration for Kubernetes", func() {
			customResources := &kruizev1alpha1.ResourceConfig{
				PersistentVolume: &kruizev1alpha1.PersistentVolumeConfig{
					PVStorageSize:    "3Gi",
					PVCStorageSize:   "2Gi",
					StorageClassName: "k8s-storage",
					HostPath:         "/k8s/custom/path",
					AccessModes:      []string{"ReadWriteMany"},
				},
			}
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", constants.ClusterTypeMinikube, customResources, getTestContext())

			clusterResources := generator.KubernetesClusterScopedResources()

			// Find the PV
			pv := findPersistentVolume(clusterResources)
			Expect(pv).NotTo(BeNil(), "PersistentVolume should exist")
			Expect(pv.Spec.Capacity.Storage().String()).To(Equal("3Gi"))
			Expect(pv.Spec.StorageClassName).To(Equal("k8s-storage"))
			Expect(pv.Spec.PersistentVolumeSource.HostPath.Path).To(Equal("/k8s/custom/path"))
			Expect(pv.Spec.AccessModes).To(Equal([]corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}))

			// Find the PVC
			pvc := findPersistentVolumeClaim(clusterResources)
			Expect(pvc).NotTo(BeNil(), "PersistentVolumeClaim should exist")
			Expect(pvc.Spec.Resources.Requests.Storage().String()).To(Equal("2Gi"))
			Expect(*pvc.Spec.StorageClassName).To(Equal("k8s-storage"))
			Expect(pvc.Spec.AccessModes).To(Equal([]corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}))
		})

		It("should fallback PVCStorageSize to PVStorageSize when PVCStorageSize is omitted for OpenShift", func() {
			customResources := &kruizev1alpha1.ResourceConfig{
				PersistentVolume: &kruizev1alpha1.PersistentVolumeConfig{
					PVStorageSize:    "5Gi",
					// PVCStorageSize is intentionally omitted
					StorageClassName: "fallback-storage",
					HostPath:         "/fallback/path",
				},
			}
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", constants.ClusterTypeOpenShift, customResources, getTestContext())

			clusterResources := generator.ClusterScopedResources()

			// Find the PV
			pv := findPersistentVolume(clusterResources)
			Expect(pv).NotTo(BeNil(), "PersistentVolume should exist")
			Expect(pv.Spec.Capacity.Storage().String()).To(Equal("5Gi"))

			// Find the PVC and verify it uses PVStorageSize
			pvc := findPersistentVolumeClaim(clusterResources)
			Expect(pvc).NotTo(BeNil(), "PersistentVolumeClaim should exist")
			// PVC should use PVStorageSize since PVCStorageSize was not specified
			Expect(pvc.Spec.Resources.Requests.Storage().String()).To(Equal("5Gi"))
		})

		It("should fallback PVCStorageSize to PVStorageSize when PVCStorageSize is omitted for Kubernetes", func() {
			customResources := &kruizev1alpha1.ResourceConfig{
				PersistentVolume: &kruizev1alpha1.PersistentVolumeConfig{
					PVStorageSize: "4Gi",
					// PVCStorageSize is intentionally omitted
					StorageClassName: "k8s-fallback",
					HostPath:         "/k8s/fallback",
				},
			}
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", constants.ClusterTypeMinikube, customResources, getTestContext())

			clusterResources := generator.KubernetesClusterScopedResources()

			// Find the PV
			pv := findPersistentVolume(clusterResources)
			Expect(pv).NotTo(BeNil(), "PersistentVolume should exist")
			Expect(pv.Spec.Capacity.Storage().String()).To(Equal("4Gi"))

			// Find the PVC and verify it uses PVStorageSize
			pvc := findPersistentVolumeClaim(clusterResources)
			Expect(pvc).NotTo(BeNil(), "PersistentVolumeClaim should exist")
			// PVC should use PVStorageSize since PVCStorageSize was not specified
			Expect(pvc.Spec.Resources.Requests.Storage().String()).To(Equal("4Gi"))
		})

		It("should use default PV/PVC configuration when ResourceConfig is nil for OpenShift", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", constants.ClusterTypeOpenShift, nil, getTestContext())

			clusterResources := generator.ClusterScopedResources()

			// Find the PV
			pv := findPersistentVolume(clusterResources)
			Expect(pv).NotTo(BeNil(), "PersistentVolume should exist")
			Expect(pv.Spec.Capacity.Storage().String()).To(Equal("500Mi"))
			Expect(pv.Spec.StorageClassName).To(Equal("manual"))
			Expect(pv.Spec.PersistentVolumeSource.HostPath.Path).To(Equal("/mnt/data"))
			Expect(pv.Spec.AccessModes).To(Equal([]corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}))

			// Find the PVC
			pvc := findPersistentVolumeClaim(clusterResources)
			Expect(pvc).NotTo(BeNil(), "PersistentVolumeClaim should exist")
			Expect(pvc.Spec.Resources.Requests.Storage().String()).To(Equal("500Mi"))
			Expect(*pvc.Spec.StorageClassName).To(Equal("manual"))
			Expect(pvc.Spec.AccessModes).To(Equal([]corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany}))
		})

		It("should use default PV/PVC configuration when ResourceConfig is nil for Kubernetes", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", constants.ClusterTypeMinikube, nil, getTestContext())

			clusterResources := generator.KubernetesClusterScopedResources()

			// Find the PV
			pv := findPersistentVolume(clusterResources)
			Expect(pv).NotTo(BeNil(), "PersistentVolume should exist")
			Expect(pv.Spec.Capacity.Storage().String()).To(Equal("1Gi"))
			Expect(pv.Spec.StorageClassName).To(Equal("manual"))
			Expect(pv.Spec.PersistentVolumeSource.HostPath.Path).To(Equal("/tmp/data"))
			Expect(pv.Spec.AccessModes).To(Equal([]corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}))

			// Find the PVC
			pvc := findPersistentVolumeClaim(clusterResources)
			Expect(pvc).NotTo(BeNil(), "PersistentVolumeClaim should exist")
			Expect(pvc.Spec.Resources.Requests.Storage().String()).To(Equal("1Gi"))
			Expect(*pvc.Spec.StorageClassName).To(Equal("manual"))
			Expect(pvc.Spec.AccessModes).To(Equal([]corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}))
		})
	})

	Context("Route and service creation", func() {
		It("should generate routes for OpenShift", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", constants.ClusterTypeOpenShift, nil, getTestContext())

			namespacedResources := generator.NamespacedResources()

			// Check for Route resources
			var hasKruizeRoute, hasUIRoute bool
			for _, resource := range namespacedResources {
				kind := resource.GetObjectKind().GroupVersionKind().Kind
				name := resource.GetName()

				if kind == "Route" && name == "kruize" {
					hasKruizeRoute = true
				}
				if kind == "Route" && name == "kruize-ui-nginx-service" {
					hasUIRoute = true
				}
			}

			Expect(hasKruizeRoute).To(BeTrue(), "Kruize route should be generated")
			Expect(hasUIRoute).To(BeTrue(), "Kruize UI route should be generated")
		})

		It("should generate services for all cluster types", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", constants.ClusterTypeOpenShift, nil, getTestContext())

			namespacedResources := generator.NamespacedResources()

			// Check for Service resources
			var hasKruizeService, hasDBService, hasUIService bool
			for _, resource := range namespacedResources {
				kind := resource.GetObjectKind().GroupVersionKind().Kind
				name := resource.GetName()

				if kind == "Service" && name == "kruize" {
					hasKruizeService = true
				}
				if kind == "Service" && name == "kruize-db-service" {
					hasDBService = true
				}
				if kind == "Service" && name == "kruize-ui-nginx-service" {
					hasUIService = true
				}
			}

			Expect(hasKruizeService).To(BeTrue(), "Kruize service should be generated")
			Expect(hasDBService).To(BeTrue(), "Kruize DB service should be generated")
			Expect(hasUIService).To(BeTrue(), "Kruize UI service should be generated")
		})
	})

	Context("Kruize endpoints validation", func() {
		It("should generate service with correct ports for Kruize", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", constants.ClusterTypeOpenShift, nil, getTestContext())

			namespacedResources := generator.NamespacedResources()

			// Find the Kruize service
			var kruizeService *corev1.Service
			for _, resource := range namespacedResources {
				if resource.GetObjectKind().GroupVersionKind().Kind == "Service" && resource.GetName() == "kruize" {
					var ok bool
					kruizeService, ok = resource.(*corev1.Service)
					Expect(ok).To(BeTrue(), "Resource should be a valid Service")
					break
				}
			}

			Expect(kruizeService).NotTo(BeNil(), "Kruize service should exist")
			Expect(kruizeService.Spec.Ports).NotTo(BeEmpty(), "Service should have ports defined")

			// Verify the service has the expected port
			var hasKruizePort bool
			for _, port := range kruizeService.Spec.Ports {
				if port.Name == "kruize-port" {
					hasKruizePort = true
					Expect(port.Port).To(Equal(int32(8080)))
				}
			}
			Expect(hasKruizePort).To(BeTrue(), "Service should have kruize-port defined")
		})

		It("should generate service with correct ports for Kruize UI", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", constants.ClusterTypeOpenShift, nil, getTestContext())

			namespacedResources := generator.NamespacedResources()

			// Find the Kruize UI service
			var kruizeUIService *corev1.Service
			for _, resource := range namespacedResources {
				if resource.GetObjectKind().GroupVersionKind().Kind == "Service" && resource.GetName() == "kruize-ui-nginx-service" {
					var ok bool
					kruizeUIService, ok = resource.(*corev1.Service)
					Expect(ok).To(BeTrue(), "Resource should be a valid Service")
					break
				}
			}

			Expect(kruizeUIService).NotTo(BeNil(), "Kruize UI service should exist")
			Expect(kruizeUIService.Spec.Ports).NotTo(BeEmpty(), "Service should have ports defined")

			// Verify the service has the expected port
			var hasUIPort bool
			for _, port := range kruizeUIService.Spec.Ports {
				if port.Name == "http" {
					hasUIPort = true
					Expect(port.Port).To(Equal(int32(8080)))
				}
			}
			Expect(hasUIPort).To(BeTrue(), "Service should have kruize-ui http port defined")
		})

		It("should generate service with correct ports for Kruize DB", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", constants.ClusterTypeOpenShift, nil, getTestContext())

			namespacedResources := generator.NamespacedResources()

			// Find the Kruize DB service
			var kruizeDBService *corev1.Service
			for _, resource := range namespacedResources {
				if resource.GetObjectKind().GroupVersionKind().Kind == "Service" && resource.GetName() == "kruize-db-service" {
					var ok bool
					kruizeDBService, ok = resource.(*corev1.Service)
					Expect(ok).To(BeTrue(), "Resource should be a valid Service")
					break
				}
			}

			Expect(kruizeDBService).NotTo(BeNil(), "Kruize DB service should exist")
			Expect(kruizeDBService.Spec.Ports).NotTo(BeEmpty(), "Service should have ports defined")

			// Verify the service has the expected port
			var hasDBPort bool
			for _, port := range kruizeDBService.Spec.Ports {
				if port.Name == "kruize-db-port" {
					hasDBPort = true
					Expect(port.Port).To(Equal(int32(5432)))
				}
			}
			Expect(hasDBPort).To(BeTrue(), "Service should have kruize-db-port defined")
		})

		It("should generate Kruize service with NodePort type", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", constants.ClusterTypeOpenShift, nil, getTestContext())

			namespacedResources := generator.NamespacedResources()

			// Find the Kruize service
			var kruizeService *corev1.Service
			for _, resource := range namespacedResources {
				if resource.GetObjectKind().GroupVersionKind().Kind == "Service" && resource.GetName() == "kruize" {
					var ok bool
					kruizeService, ok = resource.(*corev1.Service)
					Expect(ok).To(BeTrue(), "Resource should be a valid Service")
					break
				}
			}

			Expect(kruizeService).NotTo(BeNil(), "Kruize service should exist")
			Expect(kruizeService.Spec.Type).To(Equal(corev1.ServiceTypeNodePort), "Kruize service should be NodePort type")
		})

		It("should generate Kruize UI service with NodePort type", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", constants.ClusterTypeOpenShift, nil, getTestContext())

			namespacedResources := generator.NamespacedResources()

			// Find the Kruize UI service
			var kruizeUIService *corev1.Service
			for _, resource := range namespacedResources {
				if resource.GetObjectKind().GroupVersionKind().Kind == "Service" && resource.GetName() == "kruize-ui-nginx-service" {
					var ok bool
					kruizeUIService, ok = resource.(*corev1.Service)
					Expect(ok).To(BeTrue(), "Resource should be a valid Service")
					break
				}
			}

			Expect(kruizeUIService).NotTo(BeNil(), "Kruize UI service should exist")
			Expect(kruizeUIService.Spec.Type).To(Equal(corev1.ServiceTypeNodePort), "Kruize UI service should be NodePort type")
		})

		It("should generate Kruize DB service with ClusterIP type", func() {
			generator := utils.NewKruizeResourceGenerator("test-namespace", "", "", constants.ClusterTypeOpenShift, nil, getTestContext())

			namespacedResources := generator.NamespacedResources()

			// Find the Kruize DB service
			var kruizeDBService *corev1.Service
			for _, resource := range namespacedResources {
				if resource.GetObjectKind().GroupVersionKind().Kind == "Service" && resource.GetName() == "kruize-db-service" {
					var ok bool
					kruizeDBService, ok = resource.(*corev1.Service)
					Expect(ok).To(BeTrue(), "Resource should be a valid Service")
					break
				}
			}

			Expect(kruizeDBService).NotTo(BeNil(), "Kruize DB service should exist")
			Expect(kruizeDBService.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP), "Kruize DB service should be ClusterIP type")
		})
	})

    Context("Image defaulting behavior", func() {
    	It("should use default images when not specified", func() {
    		namespace := "test-default-images"
    		clusterType := constants.ClusterTypeMinikube

    		By("creating a generator with empty image fields")
    		generator := utils.NewKruizeResourceGenerator(namespace, "", "", clusterType, nil, getTestContext())

    		By("verifying the generator uses the default-image helpers")
    		// This test verifies that the generator is wired to use the default-image helpers
    		Expect(generator.Autotune_image).To(Equal(constants.GetDefaultAutotuneImage()),
    			"Generator should default Autotune_image when empty")
    		Expect(generator.Autotune_ui_image).To(Equal(constants.GetDefaultUIImage()),
    			"Generator should default Autotune_ui_image when empty")

    		By("verifying the generated resources use default images")
    		namespacedResources := generator.KubernetesNamespacedResources()

    		// Find and verify Kruize deployment using helper
    		kruizeDeployment := findTypedResource[*appsv1.Deployment](namespacedResources, "kruize", "app", "kruize")
    		Expect(kruizeDeployment).NotTo(BeNil(), "Kruize deployment should be generated")
    		Expect(kruizeDeployment.Spec.Template.Spec.Containers).NotTo(BeEmpty())
    		
    		// Find the kruize (autotune) container by name using helper
    		kruizeContainer := findContainerByName(kruizeDeployment.Spec.Template.Spec.Containers, "kruize")
    		Expect(kruizeContainer).NotTo(BeNil(), "Kruize container should exist in deployment")
    		Expect(kruizeContainer.Image).To(Equal(constants.GetDefaultAutotuneImage()),
    			"Kruize deployment should use default Autotune image")

    		// Find and verify UI pod using helper
    		kruizeUIPod := findTypedResource[*corev1.Pod](namespacedResources, "kruize-ui-nginx-pod", "app", "kruize-ui-nginx")
    		Expect(kruizeUIPod).NotTo(BeNil(), "Kruize UI pod should be generated")
    		Expect(kruizeUIPod.Spec.Containers).NotTo(BeEmpty())
    		
    		// Find the kruize-ui-nginx container by name using helper
    		uiContainer := findContainerByName(kruizeUIPod.Spec.Containers, "kruize-ui-nginx-container")
    		Expect(uiContainer).NotTo(BeNil(), "Kruize UI container should exist in pod")
    		Expect(uiContainer.Image).To(Equal(constants.GetDefaultUIImage()),
    			"Kruize UI pod should use default UI image")
    	})

    	It("should use custom images when specified", func() {
    		namespace := "test-custom-images"
    		clusterType := constants.ClusterTypeMinikube
    		customAutotuneImage := "custom.registry/autotune:custom-tag"
    		customUIImage := "custom.registry/ui:custom-tag"

    		By("creating a generator with custom image values")
    		generator := utils.NewKruizeResourceGenerator(namespace, customAutotuneImage, customUIImage, clusterType, nil, getTestContext())

    		By("verifying the generator uses the provided custom images")
    		Expect(generator.Autotune_image).To(Equal(customAutotuneImage),
    			"Generator should use provided Autotune_image")
    		Expect(generator.Autotune_ui_image).To(Equal(customUIImage),
    			"Generator should use provided Autotune_ui_image")
    	})
    })

    var _ = Describe("Metrics Auth Integration", func() {
        var client *http.Client

        BeforeEach(func() {
            // Create a client that ignores self-signed certs (standard for envtest)
            tr := &http.Transport{
                TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
            }
            client = &http.Client{Transport: tr}
        })

        Context("when accessing the metrics endpoint", func() {
            It("should reject unauthorized requests", func() {
                By("calling the metrics endpoint until it is ready")

                testMode := os.Getenv("KRUIZE_TEST_MODE")

                if testMode == "true" {
                    Eventually(func() (int, error) {
                        metricsURL := fmt.Sprintf("https://%s/metrics", utils.LocalMetricsAddr)
                        resp, err := client.Get(metricsURL)
                        if err != nil {
                            return 0, err // Return error to trigger a retry
                        }
                        defer resp.Body.Close()
                        return resp.StatusCode, nil
                    }, "10s", "1s").Should(SatisfyAny(
                        Equal(http.StatusUnauthorized),
                        Equal(http.StatusForbidden),
                        Equal(http.StatusInternalServerError),
                    ), "The metrics server should eventually be reachable and reject the unauthorized request in envtest")
                } else {
                    // In real clusters, 500 usually indicates misconfiguration, so do not accept it
                    Eventually(func() (int, error) {
                        metricsURL := fmt.Sprintf("https://%s/metrics", utils.LocalMetricsAddr)
                        resp, err := client.Get(metricsURL)
                        if err != nil {
                            return 0, err // Return error to trigger a retry
                        }
                        defer resp.Body.Close()
                        return resp.StatusCode, nil
                    }, "10s", "1s").Should(SatisfyAny(
                        Equal(http.StatusUnauthorized),
                        Equal(http.StatusForbidden),
                    ), "The metrics server should eventually be reachable and reject the unauthorized request in real-cluster mode")
                }
            })

            It("should fail when an invalid token is provided", func() {
                By("waiting for the metrics server to be ready")
                testMode := os.Getenv("KRUIZE_TEST_MODE")

                // Wait for metrics server readiness before testing invalid-token behavior
                if testMode == "true" {
                    Eventually(func() (int, error) {
                        metricsURL := fmt.Sprintf("https://%s/metrics", utils.LocalMetricsAddr)
                        resp, err := client.Get(metricsURL)
                        if err != nil {
                            return 0, err
                        }
                        defer resp.Body.Close()
                        return resp.StatusCode, nil
                    }, "10s", "1s").Should(SatisfyAny(
                        Equal(http.StatusUnauthorized),
                        Equal(http.StatusForbidden),
                        Equal(http.StatusInternalServerError),
                    ), "The metrics server should be reachable before testing invalid-token behavior")
                } else {
                    Eventually(func() (int, error) {
                        metricsURL := fmt.Sprintf("https://%s/metrics", utils.LocalMetricsAddr)
                        resp, err := client.Get(metricsURL)
                        if err != nil {
                            return 0, err
                        }
                        defer resp.Body.Close()
                        return resp.StatusCode, nil
                    }, "10s", "1s").Should(SatisfyAny(
                        Equal(http.StatusUnauthorized),
                        Equal(http.StatusForbidden),
                    ), "The metrics server should be reachable before testing invalid-token behavior")
                }

                By("executing the request with an invalid bearer token and readiness handling")
                metricsURL := fmt.Sprintf("https://%s/metrics", utils.LocalMetricsAddr)
                var resp *http.Response
                Eventually(func() error {
                    req, err := http.NewRequest("GET", metricsURL, nil)
                    if err != nil {
                        return err
                    }
                    req.Header.Set("Authorization", "Bearer invalid-token")
                    
                    resp, err = client.Do(req)
                    if err != nil {
                        return err // Retry on connection errors
                    }
                    return nil
                }, "10s", "1s").Should(Succeed(), "Should eventually connect to metrics server with invalid token")
                defer resp.Body.Close()

                By("verifying the metrics server intercepted the request")
                if os.Getenv("KRUIZE_TEST_MODE") == "true" {
                    // In envtest mode we explicitly expect the known 500 behavior,
                    // rather than treating any 500 as success.
                    Expect(resp.StatusCode).To(
                        Equal(http.StatusInternalServerError),
                        "Expected 500 from envtest when handling unauthenticated request",
                    )
                } else {
                    Expect(resp.StatusCode).To(SatisfyAny(
                        Equal(http.StatusUnauthorized),
                        Equal(http.StatusForbidden),
                    ), "Expected 401 or 403 for unauthenticated request")
                }
            })

            It("should have the metrics server running and reachable", func() {
                checkMetricsReachable := func() error {
                    metricsURL := fmt.Sprintf("https://%s/metrics", utils.LocalMetricsAddr)
                    resp, err := client.Get(metricsURL)
                    if err != nil {
                        return err
                    }
                    defer resp.Body.Close()

                    // Reachability test: getting any HTTP response proves the server is listening
                    if resp == nil {
                        return fmt.Errorf("expected non-nil response from metrics endpoint")
                    }
                    if resp.StatusCode == 0 {
                        return fmt.Errorf("expected non-zero HTTP status code from metrics endpoint")
                    }

                    return nil
                }

                By("waiting for the metrics server to become reachable on the secure port")
                Eventually(checkMetricsReachable).Should(Succeed())

                By("verifying the metrics server remains reachable on the secure port")
                Consistently(checkMetricsReachable).Should(Succeed())
            })
        })
    })
})
