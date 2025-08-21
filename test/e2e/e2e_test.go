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

package e2e

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kruize/kruize-operator/test/utils"
)

const namespace = "kruize-operator-system"

var _ = Describe("controller", Ordered, func() {
	BeforeAll(func() {
		By("installing prometheus operator")
		Expect(utils.InstallPrometheusOperator()).To(Succeed())

		By("installing the cert-manager")
		Expect(utils.InstallCertManager()).To(Succeed())

		By("creating manager namespace")
		cmd := exec.Command("kubectl", "create", "ns", namespace)
		_, _ = utils.Run(cmd)
	})

	AfterAll(func() {
		By("uninstalling the Prometheus manager bundle")
		utils.UninstallPrometheusOperator()

		By("uninstalling the cert-manager bundle")
		utils.UninstallCertManager()

		By("removing manager namespace")
		cmd := exec.Command("kubectl", "delete", "ns", namespace)
		_, _ = utils.Run(cmd)
	})

	Context("Operator", func() {
		It("should run successfully", func() {
			var controllerPodName string
			var err error

			// projectimage stores the name of the image used in the example
			var projectimage = "example.com/kruize-operator:v0.0.1"

			By("building the manager(Operator) image")
			cmd := exec.Command("make", "docker-build", fmt.Sprintf("IMG=%s", projectimage))
			_, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("loading the the manager(Operator) image on Kind")
			err = utils.LoadImageToKindClusterWithName(projectimage)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("installing CRDs")
			cmd = exec.Command("make", "install")
			_, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("deploying the controller-manager")
			cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", projectimage))
			_, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("validating that the controller-manager pod is running as expected")
			verifyControllerUp := func() error {
				// Get pod name

				cmd = exec.Command("kubectl", "get",
					"pods", "-l", "control-plane=controller-manager",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", namespace,
				)

				podOutput, err := utils.Run(cmd)
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				podNames := utils.GetNonEmptyLines(string(podOutput))
				if len(podNames) != 1 {
					return fmt.Errorf("expect 1 controller pods running, but got %d", len(podNames))
				}
				controllerPodName = podNames[0]
				ExpectWithOffset(2, controllerPodName).Should(ContainSubstring("controller-manager"))

				// Validate pod status
				cmd = exec.Command("kubectl", "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", namespace,
				)
				status, err := utils.Run(cmd)
				ExpectWithOffset(2, err).NotTo(HaveOccurred())
				if string(status) != "Running" {
					return fmt.Errorf("controller pod in %s status", status)
				}
				return nil
			}
			EventuallyWithOffset(1, verifyControllerUp, time.Minute, time.Second).Should(Succeed())

		})

		It("should deploy Kruize components successfully", func() {
			By("creating a Kruize custom resource")
			cmd := exec.Command("kubectl", "apply", "-f", "config/samples/my_v1alpha1_kruize.yaml")
			_, err := utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("checking that openshift-tuning namespace is created")
			Eventually(func() error {
				cmd := exec.Command("kubectl", "get", "namespace", "openshift-tuning")
				_, err := utils.Run(cmd)
				return err
			}, 2*time.Minute, 5*time.Second).Should(Succeed())

			By("checking that Kruize ServiceAccount is created")
			Eventually(func() error {
				cmd := exec.Command("kubectl", "get", "serviceaccount", "kruize-sa", "-n", "openshift-tuning")
				_, err := utils.Run(cmd)
				return err
			}, 2*time.Minute, 5*time.Second).Should(Succeed())

			By("checking that ConfigMap contains correct datasource")
			Eventually(func() error {
				cmd := exec.Command("kubectl", "get", "configmap", "kruizeconfig", "-n", "openshift-tuning", "-o", "jsonpath={.data.kruizeconfigjson}")
				output, err := utils.Run(cmd)
				if err != nil {
					return err
				}
				if !strings.Contains(string(output), "prometheus-1") {
					return fmt.Errorf("datasource not found in config")
				}
				if !strings.Contains(string(output), ":9091") {
					return fmt.Errorf("correct port not found in config")
				}
				if strings.Contains(string(output), "serviceName") {
					return fmt.Errorf("serviceName should not be present in datasource config")
				}
				return nil
			}, 2*time.Minute, 5*time.Second).Should(Succeed())

			By("checking that Kruize deployment is ready")
			Eventually(func() error {
				cmd := exec.Command("kubectl", "get", "deployment", "kruize", "-n", "openshift-tuning", "-o", "jsonpath={.status.readyReplicas}")
				output, err := utils.Run(cmd)
				if err != nil {
					return err
				}
				if string(output) != "1" {
					return fmt.Errorf("deployment not ready")
				}
				return nil
			}, 5*time.Minute, 10*time.Second).Should(Succeed())

			By("checking that Kruize database is ready")
			Eventually(func() error {
				cmd := exec.Command("kubectl", "get", "deployment", "kruize-db", "-n", "openshift-tuning", "-o", "jsonpath={.status.readyReplicas}")
				output, err := utils.Run(cmd)
				if err != nil {
					return err
				}
				if string(output) != "1" {
					return fmt.Errorf("database deployment not ready")
				}
				return nil
			}, 5*time.Minute, 10*time.Second).Should(Succeed())

			By("verifying Kruize API is responding")
			Eventually(func() error {
				cmd := exec.Command("kubectl", "exec", "deployment/kruize", "-n", "openshift-tuning", "--", "curl", "-s", "localhost:8080/health")
				_, err := utils.Run(cmd)
				return err
			}, 3*time.Minute, 10*time.Second).Should(Succeed())

			By("cleaning up the Kruize custom resource")
			cmd = exec.Command("kubectl", "delete", "-f", "config/samples/my_v1alpha1_kruize.yaml")
			_, _ = utils.Run(cmd)
		})
	})
})
