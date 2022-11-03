package reconcilers

import (
	"context"
	"github.com/banzaicloud/operator-tools/pkg/prometheus"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/pkg/helpers"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

func newMonitoringReconciler(spec *opsterv1.OpenSearchCluster) (*ReconcilerContext, *TLSReconciler) {
	reconcilerContext := NewReconcilerContext(spec.Spec.NodePools)
	underTest := NewTLSReconciler(
		k8sClient,
		context.Background(),
		&reconcilerContext,
		spec,
	)
	underTest.pki = helpers.NewMockPKI()
	return &reconcilerContext, underTest
}

var _ = Describe("TLS Controller", func() {

	// Define utility constants for object names and testing timeouts/durations and intervals.
	const (
		timeout  = time.Second * 30
		interval = time.Second * 1
	)

	Context("When Cluster is creating with monitoring enbale", func() {
		It("should create a cluster and add ServiceMonitor ", func() {
			clusterName := "monitoring-plugin-test"
			serviceMonitor_name := clusterName + "-monitor"
			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuuid"},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{Monitoring: opsterv1.MonitoringStuck{
						Enable: true,
					}},
				}}
			Expect(CreateNamespace(k8sClient, clusterName)).Should(Succeed())
			reconcilerContext, underTest := newTLSReconciler(&spec)
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())
			Expect(reconcilerContext.Volumes).Should(HaveLen(2))
			Expect(reconcilerContext.VolumeMounts).Should(HaveLen(2))

			//expect that at least 1 plugins in the list
			Expect(spec.Spec.General.PluginsList).To(Not(nil))

			Eventually(func() bool {
				MonitorService := prometheus.ServiceMonitor{}
				err = k8sClient.Get(context.Background(), client.ObjectKey{Name: serviceMonitor_name, Namespace: clusterName}, &MonitorService)
				if err != nil {
					return false
				}
				return err == nil
			}, timeout, interval).Should(BeTrue())

		})
	})

	Context("When Cluster is creating with monitoring enbale with OfflinePLugin provided", func() {
		It("add OfflinePLugin to PluginList ", func() {
			clusterName := "monitoring-test"
			serviceMonitor_name := clusterName + "-monitor"
			spec := opsterv1.OpenSearchCluster{
				ObjectMeta: metav1.ObjectMeta{Name: clusterName, Namespace: clusterName, UID: "dummyuid"},
				Spec: opsterv1.ClusterSpec{
					General: opsterv1.GeneralConfig{Monitoring: opsterv1.MonitoringStuck{
						Enable:        true,
						OfflinePlugin: "http://Thats_a_offline_URL_example",
					}},
				}}
			Expect(CreateNamespace(k8sClient, clusterName)).Should(Succeed())
			reconcilerContext, underTest := newTLSReconciler(&spec)
			_, err := underTest.Reconcile()
			Expect(err).ToNot(HaveOccurred())
			Expect(reconcilerContext.Volumes).Should(HaveLen(2))
			Expect(reconcilerContext.VolumeMounts).Should(HaveLen(2))

			//expect that at least 1 plugins in the list
			Expect(spec.Spec.General.PluginsList[0]).To(Equal("http://Thats_a_offline_URL_example"))

			Eventually(func() bool {
				MonitorService := prometheus.ServiceMonitor{}
				err = k8sClient.Get(context.Background(), client.ObjectKey{Name: serviceMonitor_name, Namespace: clusterName}, &MonitorService)
				if err != nil {
					return false
				}
				return err == nil
			}, timeout, interval).Should(BeTrue())

		})
	})

})
