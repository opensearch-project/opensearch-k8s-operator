package conditions

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestConditions(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Conditions Helper Suite")
}
