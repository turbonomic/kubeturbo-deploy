package utils_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.ibm.com/turbonomic/kubeturbo-deploy/internal/utils"
)

var _ = Describe("Errors", func() {
	Describe("ReturnOnError", func() {
		When("All functions return nil", func() {
			It("Returns nil", func() {

				f1_invoked := false
				f2_invoked := false

				f1 := func() error {
					f1_invoked = true
					return nil
				}

				f2 := func() error {
					f2_invoked = true
					return nil
				}

				result := utils.ReturnOnError(f1, f2)

				Expect(result).To(BeNil())
				Expect(f1_invoked).To(BeTrue())
				Expect(f2_invoked).To(BeTrue())
			})
		})

		When("A function returns an error", func() {
			It("Returns the error immediately", func() {

				f2_invoked := false
				err := fmt.Errorf("An error occurred")

				f1 := func() error {
					return err
				}

				f2 := func() error {
					f2_invoked = true
					return nil
				}

				result := utils.ReturnOnError(f1, f2)

				Expect(result).To(Equal(err))
				Expect(f2_invoked).To(BeFalse())
			})
		})

		When("No functions are passed in", func() {
			It("Returns nil", func() {
				result := utils.ReturnOnError()

				Expect(result).To(BeNil())
			})
		})
	})
})
