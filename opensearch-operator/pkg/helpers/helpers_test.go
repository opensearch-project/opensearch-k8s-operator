package helpers

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/resource"
)

var _ = Describe("JVM Heap Size Functions", func() {
	Describe("AppendJvmHeapSizeSettings", func() {
		Context("when JVM string already contains Xmx", func() {
			It("should return the original JVM string unchanged", func() {
				jvm := "-XX:+UseG1GC -Xmx2g -XX:MaxDirectMemorySize=1g"
				heapSizeSettings := "-Xms1g -Xmx2g"

				result := AppendJvmHeapSizeSettings(jvm, heapSizeSettings)

				Expect(result).To(Equal(jvm))
			})
		})

		Context("when JVM string already contains Xms", func() {
			It("should return the original JVM string unchanged", func() {
				jvm := "-XX:+UseG1GC -Xms1g -XX:MaxDirectMemorySize=1g"
				heapSizeSettings := "-Xms1g -Xmx2g"

				result := AppendJvmHeapSizeSettings(jvm, heapSizeSettings)

				Expect(result).To(Equal(jvm))
			})
		})

		Context("when JVM string is empty", func() {
			It("should return only the heap size settings", func() {
				jvm := ""
				heapSizeSettings := "-Xmx1g -Xms1g"

				result := AppendJvmHeapSizeSettings(jvm, heapSizeSettings)

				Expect(result).To(Equal(heapSizeSettings))
			})
		})

		Context("when JVM string does not contain Xmx or Xms", func() {
			It("should append the heap size settings", func() {
				jvm := "-XX:+UseG1GC -XX:MaxDirectMemorySize=1g"
				heapSizeSettings := "-Xmx1g -Xms1g"
				expected := "-XX:+UseG1GC -XX:MaxDirectMemorySize=1g -Xmx1g -Xms1g"

				result := AppendJvmHeapSizeSettings(jvm, heapSizeSettings)

				Expect(result).To(Equal(expected))
			})
		})
	})

	Describe("CalculateJvmHeapSizeSettings", func() {
		Context("when memory request is nil", func() {
			It("should return default 512M for both Xms and Xmx", func() {
				result := CalculateJvmHeapSizeSettings(nil)

				Expect(result).To(Equal("-Xms512M -Xmx512M"))
			})
		})

		Context("when memory request is zero", func() {
			It("should return default 512M for both Xms and Xmx", func() {
				memoryRequest := resource.MustParse("0")

				result := CalculateJvmHeapSizeSettings(&memoryRequest)

				Expect(result).To(Equal("-Xms512M -Xmx512M"))
			})
		})

		Context("when memory request is provided", func() {
			It("should calculate both Xms and Xmx from request", func() {
				memoryRequest := resource.MustParse("2Gi")

				result := CalculateJvmHeapSizeSettings(&memoryRequest)

				Expect(result).To(Equal("-Xms1024M -Xmx1024M"))
			})
		})
	})
})
