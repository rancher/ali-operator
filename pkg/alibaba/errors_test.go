package alibaba

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("IsNotFound", func() {
	It("should return true for errors containing ErrorClusterNotFound", func() {
		err := errors.New("some api error with ErrorClusterNotFound inside")
		Expect(IsNotFound(err)).To(BeTrue())
	})

	It("should return false for other errors", func() {
		err := errors.New("some other api error")
		Expect(IsNotFound(err)).To(BeFalse())
	})

	It("should return false for nil error", func() {
		Expect(IsNotFound(nil)).To(BeFalse())
	})
})
