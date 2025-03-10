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
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.ibm.com/turbonomic/kubeturbo-deploy/test/utils"
)

var _ = Describe("Operator lifecycle", Ordered, func() {
	var err error
	var cmd *exec.Cmd

	var before_test_k8s_context string
	test_k8s_context := "kind-" + utils.KIND_CLUSTER

	// projectImage stores the name of the image used in the test
	var PROJECT_IMAGE = utils.REGISTRY + "/" + utils.OPERATOR_NAME + ":" + utils.VERSION

	// path to the generated YAML bundle for the Kubeturbo operator
	base, _ := os.Getwd()
	YAML_BUNDLE_DIR := filepath.Join(base, "../../deploy/kubeturbo_operator_yamls")
	YAML_BUNDLE_PATH, _ := filepath.Abs(filepath.Join(YAML_BUNDLE_DIR, "operator-bundle.yaml"))

	BeforeAll(func() {
		By("Check or create the kind cluster")
		os.Setenv("KUBECONFIG_STR", utils.KIND_KUBECONFIG_STR)
		cmd = exec.Command("make", "create-kind-cluster")
		_, _ = utils.Run(cmd)

		By("Get current K8s cluster context")
		before_test_k8s_context = utils.CurrentK8sContext()

		By("Switch to the Kind cluster")
		err = utils.SwitchToContext(test_k8s_context)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())

		By("Build test image")
		cmd = exec.Command("make", "docker-build", fmt.Sprintf("IMG=%s", PROJECT_IMAGE))
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())

		By("Load image to Kind cluster")
		err = utils.LoadImageToKindClusterWithName(PROJECT_IMAGE)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		By("Switch back to the K8s cluster before test")
		if before_test_k8s_context != "" &&
			before_test_k8s_context != test_k8s_context {
			By("revert k8s context change")
			err := utils.SwitchToContext(before_test_k8s_context)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())
		}
	})

	Context("YAML approach", func() {
		It("Generate bundle", func() {
			By("Create namespace")
			os.Setenv("NAMESPACE", utils.NAMESPACE)
			cmd = exec.Command(utils.KUBECTL, "create", "ns", utils.NAMESPACE)
			_, _ = utils.Run(cmd)

			cmd = exec.Command("make", "export_operator_yaml_bundle")
			_, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("Install bundle")
			cmd = exec.Command(utils.KUBECTL, "apply", "-f", YAML_BUNDLE_PATH)
			_, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("Validate installation")
			assets := utils.OPERATOR_ASSETS

			By("Verify Asset creation")
			ExpectWithOffset(1, utils.VerifyK8sAssets(assets)).NotTo(HaveOccurred())

			By("Verify ClusterRoleBinding relationship")
			ExpectWithOffset(1, utils.VerifyCRB(assets)).NotTo(HaveOccurred())

			By("Verify Deployment settings")
			ExpectWithOffset(1, utils.VerifyDeployment(assets)).NotTo(HaveOccurred())

			By("Uninstall bundle")
			cmd = exec.Command(utils.KUBECTL, "delete", "-f", YAML_BUNDLE_PATH, "--ignore-not-found=true")
			_, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())
		})
	})

	Context("Direct deploy approach", func() {
		It("Install CRDs", func() {
			By("Create namespace")
			os.Setenv("NAMESPACE", utils.NAMESPACE)
			cmd = exec.Command(utils.KUBECTL, "create", "ns", utils.NAMESPACE)
			_, _ = utils.Run(cmd)

			cmd = exec.Command("make", "install")
			_, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("Deploy operator")
			cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", PROJECT_IMAGE))
			_, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("Validate installation")
			assets := utils.OPERATOR_ASSETS

			By("Verify Asset creation")
			ExpectWithOffset(1, utils.VerifyK8sAssets(assets)).NotTo(HaveOccurred())

			By("Verify ClusterRoleBinding relationship")
			ExpectWithOffset(1, utils.VerifyCRB(assets)).NotTo(HaveOccurred())

			By("Verify Deployment settings")
			ExpectWithOffset(1, utils.VerifyDeployment(assets)).NotTo(HaveOccurred())

			By("Undeploy operator")
			cmd = exec.Command("make", "undeploy")
			_, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("Uninstall CRDs")
			cmd = exec.Command("make", "uninstall", "ignore-not-found=true")
			_, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())
		})
	})
})
