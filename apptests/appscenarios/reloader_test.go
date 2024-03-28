package appscenarios

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Reloader Install Test", Ordered, Label("reloader", "install"), func() {
	It("should return the name of the scenario", func() {
		r := reloader{}
		Expect(r.Name()).To(Equal("reloader"))
	})

})

var _ = Describe("Reloader Upgrade Test", Ordered, Label("reloader", "upgrade"), func() {
	It("should return the name of the scenario", func() {
		r := reloader{}
		Expect(r.Name()).To(Equal("reloader1"))
	})
})
