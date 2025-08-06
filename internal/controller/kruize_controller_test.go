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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	mydomainv1alpha1 "github.com/kruize/kruize-operator/api/v1alpha1"
)

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
		It("should handle OpenShift cluster type", func() {
			kruize := &mydomainv1alpha1.Kruize{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kruize-openshift",
					Namespace: "default",
				},
				Spec: mydomainv1alpha1.KruizeSpec{
					Cluster_type: "openshift",
					Namespace:    "openshift-tuning",
					Size:         1,
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
					Name:      "test-kruize-openshift",
					Namespace: "default",
				},
			})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should reject unsupported cluster types", func() {
			kruize := &mydomainv1alpha1.Kruize{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kruize-invalid",
					Namespace: "default",
				},
				Spec: mydomainv1alpha1.KruizeSpec{
					Cluster_type: "invalid-cluster",
					Namespace:    "test",
					Size:         1,
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
					Name:      "test-kruize-invalid",
					Namespace: "default",
				},
			})
			Expect(err).To(HaveOccurred())
		})
	})

	Context("Manifest generation", func() {
		var reconciler *KruizeReconciler

		BeforeEach(func() {
			reconciler = &KruizeReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
		})

		It("should generate valid RBAC and Config manifest", func() {
			manifest := reconciler.generateKruizeRBACAndConfigManifest("test-namespace", "openshift")

			Expect(manifest).To(ContainSubstring("kind: Namespace"))
			Expect(manifest).To(ContainSubstring("kind: ServiceAccount"))
			Expect(manifest).To(ContainSubstring("kind: ClusterRole"))
			Expect(manifest).To(ContainSubstring("kind: ConfigMap"))
		})

		It("should include correct datasource configuration", func() {
			manifest := reconciler.generateKruizeRBACAndConfigManifest("test-namespace", "openshift")

			Expect(manifest).To(ContainSubstring("prometheus-1"))
			Expect(manifest).To(ContainSubstring(":9091"))
			Expect(manifest).NotTo(ContainSubstring("serviceName"))
		})

		It("should generate valid deployment manifest", func() {
			manifest := reconciler.generateKruizeDeploymentManifest("test-namespace")

			Expect(manifest).To(ContainSubstring("kind: Deployment"))
			Expect(manifest).To(ContainSubstring("name: kruize"))
			Expect(manifest).To(ContainSubstring("serviceAccountName: kruize-sa"))
		})
	})

})
