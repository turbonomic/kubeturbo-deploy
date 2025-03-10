package utils_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"reflect"

	"github.ibm.com/turbonomic/kubeturbo-deploy/internal/utils"
)

type PointersTestStruct struct {
	String  string
	Integer int
}

var _ = Describe("Pointers", func() {
	Describe("AsPtr", func() {
		When("Argument is an integer", func() {
			It("Returns a pointer to that integer", func() {
				p := utils.AsPtr(123)

				Expect(reflect.TypeOf(p).Kind()).To(Equal(reflect.Ptr))
				Expect(*p).To(Equal(123))
			})
		})

		When("Argument is a struct", func() {
			It("Returns a pointer to that struct", func() {

				s := PointersTestStruct{
					String:  "a string",
					Integer: 123,
				}

				p := utils.AsPtr(s)

				Expect(reflect.TypeOf(p).Kind()).To(Equal(reflect.Ptr))
				Expect(p).To(Equal(&s))
			})
		})
	})
})
