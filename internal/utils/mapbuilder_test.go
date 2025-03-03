package utils_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.ibm.com/turbonomic/kubeturbo-deploy/internal/utils"
)

var _ = Describe("Mapbuilder", func() {
	When("No values are supplied to the builder", func() {
		It("Builds an empty map", func() {
			m := utils.NewMapBuilder[string, string]().Build()

			Expect(m).To(Equal(map[string]string{}))
		})
	})

	When("Values are supplied to the builder using Put()", func() {
		It("Builds a map containing the supplied values", func() {
			m := utils.NewMapBuilder[string, int]().
				Put("key1", 1).
				Put("key2", 2).
				Build()

			Expect(m).To(Equal(map[string]int{
				"key1": 1,
				"key2": 2,
			}))
		})
	})

	When("A key which already exists is supplied to Put()", func() {
		It("Builds a map containing the most recent value for that key", func() {
			m := utils.NewMapBuilder[string, int]().
				Put("key1", 1).
				Put("key1", 2).
				Build()

			Expect(m).To(Equal(map[string]int{
				"key1": 2,
			}))
		})
	})

	When("Values are supplied to the builder using PutAll()", func() {
		It("Builds a map containing those values", func() {
			m := utils.NewMapBuilder[string, int]().
				PutAll(map[string]int{
					"key1": 1,
					"key2": 2,
				}).
				Build()

			Expect(m).To(Equal(map[string]int{
				"key1": 1,
				"key2": 2,
			}))
		})
	})
})
