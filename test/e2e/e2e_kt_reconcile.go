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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	ktv1 "github.ibm.com/turbonomic/kubeturbo-deploy/api/v1"

	"github.ibm.com/turbonomic/kubeturbo-deploy/internal/constants"
	"github.ibm.com/turbonomic/kubeturbo-deploy/test/utils"
)

var _ = Describe("Test CR reconciliation", Ordered, func() {
	var err error
	var cmd *exec.Cmd
	var tmpCRFile *os.File
	var ktPodName string
	var kt ktv1.Kubeturbo

	var before_test_k8s_context string
	test_k8s_context := "kind-" + utils.KIND_CLUSTER

	// projectImage stores the name of the image used in the test
	var PROJECT_IMAGE = utils.REGISTRY + "/" + utils.OPERATOR_NAME + ":" + utils.VERSION

	// path to the generated YAML bundle for the Kubeturbo operator
	base, _ := os.Getwd()
	YAML_BUNDLE_DIR := filepath.Join(base, "../../deploy/kubeturbo_operator_yamls")
	YAML_BUNDLE_PATH, _ := filepath.Abs(filepath.Join(YAML_BUNDLE_DIR, "operator-bundle.yaml"))

	KT_CR_Name := "kubeturbo-release"
	WAIT_PERIOD := 20 * time.Second

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

		By("Create namespace")
		os.Setenv("NAMESPACE", utils.NAMESPACE)
		cmd = exec.Command(utils.KUBECTL, "create", "ns", utils.NAMESPACE)
		_, _ = utils.Run(cmd)

		By("Install Kubeturbo Operator")
		cmd = exec.Command("make", "export_operator_yaml_bundle")
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())

		cmd = exec.Command(utils.KUBECTL, "apply", "-f", YAML_BUNDLE_PATH)
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())

		// Create a temporary file in the system's default temporary directory
		tmpCRFile, err = os.CreateTemp("", "kt-cr-*.json")
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
		utils.LogMessage("INFO", "Temporary CR file created: %s\n", tmpCRFile.Name())
	})

	AfterAll(func() {
		By("Clean up")
		cmd = exec.Command(utils.KUBECTL, "delete", "-f", tmpCRFile.Name())
		_, _ = utils.Run(cmd)
		utils.LogMessage("INFO", "Temporary CR file remove: %s\n", tmpCRFile.Name())
		os.Remove(tmpCRFile.Name())

		By("Uninstall the Operator")
		cmd = exec.Command(utils.KUBECTL, "delete", "-f", YAML_BUNDLE_PATH, "--ignore-not-found=true")
		_, err = utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred())

		By("Switch back to the K8s cluster before test")
		if before_test_k8s_context != "" &&
			before_test_k8s_context != test_k8s_context {
			By("revert k8s context change")
			err := utils.SwitchToContext(before_test_k8s_context)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())
		}
	})

	Context("Create plain CR", func() {
		It("Apply CR", func() {
			version := "8.14.6"
			kt = utils.GenerateKtV1CRWithSpec(KT_CR_Name, &ktv1.KubeturboSpec{
				Image:      ktv1.KubeturboImage{Tag: &version},
				ServerMeta: ktv1.KubeturboServerMeta{Version: &version},
			})
			ExpectWithOffset(1, utils.WriteToFile(tmpCRFile, kt)).NotTo(HaveOccurred())

			cmd = exec.Command(utils.KUBECTL, "apply", "-f", tmpCRFile.Name())
			_, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("Validate installation")
			assets := utils.GenerateKubeturboAssets(kt)

			By(fmt.Sprintf("Wait %s for operator to act", WAIT_PERIOD.String()))
			time.Sleep(WAIT_PERIOD)

			By("Verify Assets creation")
			ExpectWithOffset(1, utils.VerifyK8sAssets(assets)).NotTo(HaveOccurred())

			By("Verify ClusterRoleBinding relationship")
			ExpectWithOffset(1, utils.VerifyCRB(assets)).NotTo(HaveOccurred())

			By("Verify Deployment image tag")
			ExpectWithOffset(1, utils.VerifyImageTag(assets, constants.KubeturboContainerName, version)).NotTo(HaveOccurred())

			By("Verify Deployment settings")
			ExpectWithOffset(1, utils.VerifyDeployment(assets)).NotTo(HaveOccurred())

			By("Caching running Kubeturbo pod name")
			ktPodName, err = utils.GetRunningPodByDeployName(KT_CR_Name)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())
		})
	})

	Context("Modify CR that restarts pod", func() {
		version := "8.13.1"
		targetName := "e2e-test-cluster"
		roleName := ktv1.RoleTypeReadOnly
		It("Apply CR", func() {
			kt.Spec = ktv1.KubeturboSpec{
				Image:        ktv1.KubeturboImage{Tag: &version},
				ServerMeta:   ktv1.KubeturboServerMeta{Version: &version},
				TargetConfig: ktv1.KubeturboTargetConfig{TargetName: &targetName},
				RoleName:     roleName,
			}
			ExpectWithOffset(1, utils.WriteToFile(tmpCRFile, kt)).NotTo(HaveOccurred())

			cmd = exec.Command(utils.KUBECTL, "apply", "-f", tmpCRFile.Name())
			_, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("Validate changes")
			oldKtPodName := ktPodName
			assets := utils.GenerateKubeturboAssets(kt)

			By(fmt.Sprintf("Wait %s for operator to act", WAIT_PERIOD.String()))
			time.Sleep(WAIT_PERIOD)

			// expecting pod restart with a different tag
			By("Verify Deployment image tag")
			ExpectWithOffset(1, utils.VerifyImageTag(assets, constants.KubeturboContainerName, version)).NotTo(HaveOccurred())

			By("Verify Deployment settings")
			ExpectWithOffset(1, utils.VerifyDeployment(assets)).NotTo(HaveOccurred())

			By("Caching running Kubeturbo pod name")
			ktPodName, err = utils.GetRunningPodByDeployName(KT_CR_Name)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("Verify the Kubeturbo pod restarts")
			utils.LogMessage("INFO", "Pod restarts: %s -> %s ", oldKtPodName, ktPodName)
			ExpectWithOffset(1, ktPodName).NotTo(Equal(oldKtPodName))
		})
	})

	Context("Modify CR that should not restarts pod", func() {
		logLevel := 5
		roleName := ktv1.RoleTypeAdmin
		npMax := 100
		roleBinding := "e2e-test-binding"
		It("Apply CR", func() {
			kt.Spec.RoleName = roleName
			kt.Spec.Logging = ktv1.Logging{Level: &logLevel}
			kt.Spec.NodePoolSize = ktv1.NodePoolSize{Max: &npMax}
			kt.Spec.RoleBinding = roleBinding
			ExpectWithOffset(1, utils.WriteToFile(tmpCRFile, kt)).NotTo(HaveOccurred())

			cmd = exec.Command(utils.KUBECTL, "apply", "-f", tmpCRFile.Name())
			_, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By("Validate changes")
			oldKtPodName := ktPodName
			assets := utils.GenerateKubeturboAssets(kt)

			By(fmt.Sprintf("Wait %s for operator to act", WAIT_PERIOD.String()))
			time.Sleep(WAIT_PERIOD)

			// change roleName: update the clusterRole and clusterRoleBinding for Kubeturbo
			By("Verify Assets creation")
			ExpectWithOffset(1, utils.VerifyK8sAssets(assets)).NotTo(HaveOccurred())

			By("Verify ClusterRoleBinding relationship")
			ExpectWithOffset(1, utils.VerifyCRB(assets)).NotTo(HaveOccurred())

			By("Caching running Kubeturbo pod name")
			ktPodName, err = utils.GetRunningPodByDeployName(KT_CR_Name)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			// change logLevel: update the dynamic config in the configMap
			// change npMax: update the dynamic config in the configMap
			By("Verify the Kubeturbo pod not restarts")
			ExpectWithOffset(1, ktPodName).To(Equal(oldKtPodName))
		})
	})

	Context("Delete CR", func() {
		It("Delete CR", func() {
			cmd = exec.Command(utils.KUBECTL, "delete", "-f", tmpCRFile.Name())
			_, err = utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			By(fmt.Sprintf("Wait %s for operator to act", WAIT_PERIOD.String()))
			time.Sleep(WAIT_PERIOD)

			By("Verify created Kubeturbo resources are deleted")
			cmd = exec.Command(utils.KUBECTL,
				"-n", utils.NAMESPACE,
				"get", "deploy,sa,cm,clusterrole,clusterrolebinding",
				"-o", "name",
				"-l", fmt.Sprintf("%s=%s", constants.PartOfLabelKey, utils.OPERATOR_NAME),
			)
			result, err := utils.Run(cmd)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())

			// General check if all Kubeturbo resources that created
			// by the operator are deleted when we deleting the CR
			ExpectWithOffset(1, string(result)).Should(BeEmpty())
		})
	})
})
