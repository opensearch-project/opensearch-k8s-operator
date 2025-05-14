package helpers

import (
	"testing"

	"github.com/Opster/opensearch-k8s-operator/opensearch-operator/mocks/github.com/Opster/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestHelpers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Helpers Suite")
}

var _ = Describe("Helpers", func() {
	When("A STS has a pod stuck with the same revision", func() {
		It("Should do nothing", func() {
			mockClient := k8s.NewMockK8sClient(GinkgoT())

			pod := corev1.Pod{
				ObjectMeta: v1.ObjectMeta{
					Labels: map[string]string{
						stsRevisionLabel: "foo",
					},
				},
			}
			mockClient.EXPECT().GetPod("foo-0", "").Return(pod, nil)
			sts := &appsv1.StatefulSet{
				ObjectMeta: v1.ObjectMeta{Name: "foo"},
				Status:     appsv1.StatefulSetStatus{UpdateRevision: "foo"},
			}
			ok, err := DeleteStuckPodWithOlderRevision(mockClient, sts)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).NotTo(BeTrue())
		})
	})

	When("A STS has a pod stuck with a different revision", func() {
		It("Should delete it", func() {
			mockClient := k8s.NewMockK8sClient(GinkgoT())

			pod := corev1.Pod{
				ObjectMeta: v1.ObjectMeta{
					Name: "foo-0",
					Labels: map[string]string{
						stsRevisionLabel: "foo",
					},
				},
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							State: corev1.ContainerState{
								Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"},
							},
						},
					},
				},
			}
			deletedPod := corev1.Pod{
				ObjectMeta: v1.ObjectMeta{
					Name:      "foo-0",
					Namespace: "",
				}}
			mockClient.EXPECT().GetPod("foo-0", "").Return(pod, nil)
			mockClient.EXPECT().DeletePod(&deletedPod).Return(nil)
			sts := &appsv1.StatefulSet{
				ObjectMeta: v1.ObjectMeta{Name: "foo"},
				Status:     appsv1.StatefulSetStatus{UpdateRevision: "bar"},
			}
			ok, err := DeleteStuckPodWithOlderRevision(mockClient, sts)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())

		})
	})
})
