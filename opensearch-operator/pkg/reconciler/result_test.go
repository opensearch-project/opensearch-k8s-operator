package reconciler

import (
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	ctrl "sigs.k8s.io/controller-runtime"
)

var _ = Describe("CombinedResult", func() {
	It("keeps the minimum RequeueAfter across sub-results", func() {
		results := &CombinedResult{}
		results.Combine(&ctrl.Result{RequeueAfter: 30 * time.Second}, nil)
		results.Combine(&ctrl.Result{RequeueAfter: 10 * time.Second}, nil)
		results.Combine(&ctrl.Result{RequeueAfter: 20 * time.Second}, nil)

		Expect(results.Result.RequeueAfter).To(Equal(10 * time.Second))
		Expect(results.Result.Requeue).To(BeFalse())
	})

	It("ORs Requeue flags so an earlier pool requeue is not lost", func() {
		results := &CombinedResult{}
		results.Combine(&ctrl.Result{Requeue: true}, nil)
		results.Combine(&ctrl.Result{Requeue: false}, nil)

		Expect(results.Result.Requeue).To(BeTrue())
	})

	It("combines errors", func() {
		results := &CombinedResult{}
		results.Combine(&ctrl.Result{}, errors.New("first"))
		results.Combine(&ctrl.Result{Requeue: true}, errors.New("second"))

		Expect(results.Err).To(HaveOccurred())
		Expect(results.Result.Requeue).To(BeTrue())
	})
})
