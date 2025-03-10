package utils_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.ibm.com/turbonomic/kubeturbo-deploy/internal/utils"
)

const (
	TestVersion = "0.0.0-SNAPSHOT"
)

var _ = Describe("Utils", func() {
	Describe("GetDefaultVersion", func() {
		When("Argument is a struct", func() {
			It("Get global version", func() {
				err := os.Setenv(utils.DefaultKubeturboVersionEnvVar, TestVersion)
				ExpectWithOffset(1, err).NotTo(HaveOccurred())
				kubeturboVersion, err := utils.GetDefaultKubeturboVersion()
				ExpectWithOffset(1, kubeturboVersion).To(Equal(TestVersion))
				ExpectWithOffset(1, err).NotTo(HaveOccurred())
			})
		})
	})
})
