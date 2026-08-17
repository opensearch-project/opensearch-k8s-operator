package reconcilers

import (
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("TerminalError", func() {
	It("wraps and detects permanent errors", func() {
		base := errors.New("invalid version")
		err := AsTerminal(base)

		Expect(IsTerminal(err)).To(BeTrue())
		Expect(IsTerminal(base)).To(BeFalse())
		Expect(errors.Is(err, base)).To(BeTrue())
		Expect(err.Error()).To(Equal("invalid version"))
	})

	It("is a no-op for nil and already-terminal errors", func() {
		Expect(AsTerminal(nil)).To(BeNil())

		once := AsTerminal(fmt.Errorf("pdb invalid"))
		twice := AsTerminal(once)
		Expect(twice).To(Equal(once))
		Expect(IsTerminal(twice)).To(BeTrue())
	})

	It("unwraps through fmt.Errorf wrapping", func() {
		err := fmt.Errorf("upgrade failed: %w", AsTerminal(ErrVersionDowngrade))
		Expect(IsTerminal(err)).To(BeTrue())
		Expect(errors.Is(err, ErrVersionDowngrade)).To(BeTrue())
	})
})
