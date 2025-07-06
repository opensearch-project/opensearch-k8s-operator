package conditions

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
)

var _ = Describe("Conditions helper", func() {
	var cluster *opsterv1.OpenSearchCluster

	BeforeEach(func() {
		cluster = &opsterv1.OpenSearchCluster{}
		// pretend controller processed generation 5
		cluster.ObjectMeta.Generation = 5
	})

	It("sets Ready condition true and observedGeneration", func() {
		SetReady(cluster, true, "TestReason", "all good")

		cond := meta.FindStatusCondition(cluster.Status.Conditions, ConditionReady)
		Expect(cond).NotTo(BeNil())
		Expect(cond.Status).To(Equal(metav1.ConditionTrue))
		Expect(cond.Reason).To(Equal("TestReason"))
		Expect(cond.Message).To(Equal("all good"))
		Expect(cluster.Status.ObservedGeneration).To(Equal(int64(5)))
	})

	It("sets Reconciling false", func() {
		SetReconciling(cluster, false, "Idle", "no work")

		cond := meta.FindStatusCondition(cluster.Status.Conditions, ConditionReconciling)
		Expect(cond).NotTo(BeNil())
		Expect(cond.Status).To(Equal(metav1.ConditionFalse))
	})

	It("overwrites existing condition keeping history", func() {
		// First set true
		SetReady(cluster, true, "Foo", "bar")
		// Save first transition
		firstTransition := meta.FindStatusCondition(cluster.Status.Conditions, ConditionReady).LastTransitionTime
		// Now set false, expect transition time to change
		SetReady(cluster, false, "Baz", "qux")
		cond := meta.FindStatusCondition(cluster.Status.Conditions, ConditionReady)
		Expect(cond.Status).To(Equal(metav1.ConditionFalse))
		Expect(cond.LastTransitionTime.Time.After(firstTransition.Time)).To(BeTrue())
	})
})
