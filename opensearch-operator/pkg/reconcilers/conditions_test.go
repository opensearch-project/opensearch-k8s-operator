package reconcilers

import (
	"errors"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	appsv1 "k8s.io/api/apps/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func convergedSts(replicas int32) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Generation: 2},
		Spec:       appsv1.StatefulSetSpec{Replicas: ptr.To(replicas)},
		Status: appsv1.StatefulSetStatus{
			ObservedGeneration: 2,
			UpdateRevision:     "rev-2",
			UpdatedReplicas:    replicas,
			ReadyReplicas:      replicas,
		},
	}
}

func convergedCluster() (opensearchv1.ClusterStatus, clusterObservation) {
	status := opensearchv1.ClusterStatus{Initialized: true}
	obs := clusterObservation{
		health:    opensearchv1.OpenSearchGreenHealth,
		nodePools: []nodePoolObservation{{component: "masters", desired: 3, sts: convergedSts(3), readyPods: 3}},
	}
	return status, obs
}

func expectCondition(conditions []metav1.Condition, conditionType string, status metav1.ConditionStatus, reason string) *metav1.Condition {
	condition := apimeta.FindStatusCondition(conditions, conditionType)
	ExpectWithOffset(1, condition).NotTo(BeNil(), "condition %s missing", conditionType)
	ExpectWithOffset(1, condition.Status).To(Equal(status))
	ExpectWithOffset(1, condition.Reason).To(Equal(reason))
	return condition
}

var _ = Describe("deriveConditions", func() {
	It("reports Ready, not progressing and not degraded for a converged cluster", func() {
		status, obs := convergedCluster()
		conditions := deriveConditions(status, obs, PassResult{})
		expectCondition(conditions, opensearchv1.ConditionReady, metav1.ConditionTrue, opensearchv1.ReasonReconciled)
		expectCondition(conditions, opensearchv1.ConditionProgressing, metav1.ConditionFalse, opensearchv1.ReasonReconciled)
		expectCondition(conditions, opensearchv1.ConditionDegraded, metav1.ConditionFalse, opensearchv1.ReasonAsExpected)
	})

	It("reports Initializing until the cluster has bootstrapped, ahead of any other reason", func() {
		status, obs := convergedCluster()
		status.Initialized = false
		status.ComponentsStatus = []opensearchv1.ComponentStatus{{Component: "Scaler", Status: "Running", Description: "masters"}}
		conditions := deriveConditions(status, obs, PassResult{})
		expectCondition(conditions, opensearchv1.ConditionReady, metav1.ConditionFalse, opensearchv1.ReasonInitializing)
		expectCondition(conditions, opensearchv1.ConditionProgressing, metav1.ConditionTrue, opensearchv1.ReasonInitializing)
	})

	DescribeTable("reports the in-flight component as Ready=False and Progressing=True",
		func(components []opensearchv1.ComponentStatus, readyReason, progressingReason, message string) {
			status, obs := convergedCluster()
			status.ComponentsStatus = components
			conditions := deriveConditions(status, obs, PassResult{})
			ready := expectCondition(conditions, opensearchv1.ConditionReady, metav1.ConditionFalse, readyReason)
			Expect(ready.Message).To(Equal(message))
			progressing := expectCondition(conditions, opensearchv1.ConditionProgressing, metav1.ConditionTrue, progressingReason)
			Expect(progressing.Message).To(Equal(message))
		},
		Entry("securityconfig job running",
			[]opensearchv1.ComponentStatus{{Component: "Securityconfig", Status: "Running"}},
			opensearchv1.ReasonSecurityConfigPending, "Securityconfig", "Running"),
		Entry("securityconfig job failed",
			[]opensearchv1.ComponentStatus{{Component: "Securityconfig", Status: "Failed", Description: "securityconfig update job failed"}},
			opensearchv1.ReasonSecurityConfigPending, "Securityconfig", "securityconfig update job failed: Failed"),
		Entry("upgrade of a node pool",
			[]opensearchv1.ComponentStatus{
				{Component: "Upgrader", Description: upgradeTargetDescription, Status: "2.19.0"},
				{Component: "Upgrader", Description: "masters", Status: upgradeStatusInProgress},
			},
			opensearchv1.ReasonUpgrading, "Upgrader", "upgrading node pool masters to 2.19.0"),
		Entry("upgrade target recorded, no pool started yet",
			[]opensearchv1.ComponentStatus{{Component: "Upgrader", Description: upgradeTargetDescription, Status: "2.19.0"}},
			opensearchv1.ReasonUpgrading, "Upgrader", "upgrading to 2.19.0"),
		Entry("scale-down drain",
			[]opensearchv1.ComponentStatus{{Component: "Scaler", Status: "Excluded", Description: "masters", Conditions: []string{"masters-2"}}},
			opensearchv1.ReasonScaling, "Scaler", "masters: Excluded (masters-2)"),
		Entry("rolling restart",
			[]opensearchv1.ComponentStatus{{Component: "RollingRestart", Status: statusInProgress}},
			opensearchv1.ReasonRollingRestart, "RollingRestart", "rolling restart in progress"),
		Entry("securityconfig ahead of an upgrade",
			[]opensearchv1.ComponentStatus{
				{Component: "Upgrader", Description: upgradeTargetDescription, Status: "2.19.0"},
				{Component: "Securityconfig", Status: "Running"},
			},
			opensearchv1.ReasonSecurityConfigPending, "Securityconfig", "Running"),
	)

	It("does not count finished or applied components as in flight", func() {
		status, obs := convergedCluster()
		status.ComponentsStatus = []opensearchv1.ComponentStatus{
			{Component: "RollingRestart", Status: statusFinished},
			{Component: "Securityconfig", Status: "Ready"},
		}
		expectCondition(deriveConditions(status, obs, PassResult{}), opensearchv1.ConditionReady, metav1.ConditionTrue, opensearchv1.ReasonReconciled)
	})

	It("reports Scaling as soon as the spec asks for a different replica count", func() {
		status, obs := convergedCluster()
		obs.nodePools[0].desired = 2
		conditions := deriveConditions(status, obs, PassResult{})
		ready := expectCondition(conditions, opensearchv1.ConditionReady, metav1.ConditionFalse, opensearchv1.ReasonScaling)
		Expect(ready.Message).To(Equal("node pool masters: 3 of 2 replicas"))
		expectCondition(conditions, opensearchv1.ConditionProgressing, metav1.ConditionTrue, "Scaler")
	})

	It("reports a terminal reconciler error as Ready=False", func() {
		status, obs := convergedCluster()
		pass := PassResult{Terminal: &ReconcilerError{Reconciler: "upgrade", Err: errors.New("version downgrade is not supported")}}
		ready := expectCondition(deriveConditions(status, obs, pass), opensearchv1.ConditionReady, metav1.ConditionFalse, opensearchv1.ReasonReconcileError)
		Expect(ready.Message).To(Equal("upgrade reconciler: version downgrade is not supported"))
	})

	DescribeTable("reports ClusterUnreachable while health cannot be read",
		func(health opensearchv1.OpenSearchHealth) {
			status, obs := convergedCluster()
			obs.health = health
			expectCondition(deriveConditions(status, obs, PassResult{}), opensearchv1.ConditionReady, metav1.ConditionFalse, opensearchv1.ReasonClusterUnreachable)
		},
		Entry("unknown", opensearchv1.OpenSearchUnknownHealth),
		Entry("never read", opensearchv1.OpenSearchHealth("")),
	)

	DescribeTable("reports PodsNotReady until every StatefulSet has converged",
		func(mutate func(*nodePoolObservation), message string) {
			status, obs := convergedCluster()
			mutate(&obs.nodePools[0])
			ready := expectCondition(deriveConditions(status, obs, PassResult{}), opensearchv1.ConditionReady, metav1.ConditionFalse, opensearchv1.ReasonPodsNotReady)
			Expect(ready.Message).To(Equal(message))
		},
		Entry("StatefulSet missing", func(p *nodePoolObservation) { p.sts = nil },
			"node pool masters: StatefulSet not created yet"),
		Entry("template change not observed", func(p *nodePoolObservation) { p.sts.Generation = 3 },
			"node pool masters: StatefulSet update not observed yet"),
		Entry("pods on the old revision", func(p *nodePoolObservation) { p.sts.Status.UpdatedReplicas = 1 },
			"node pool masters: 1/3 pods updated"),
		Entry("pod not ready", func(p *nodePoolObservation) { p.readyPods = 2 },
			"node pool masters: 2/3 pods ready"),
	)

	It("leaves Ready untouched when a pass that was cut short would report it True", func() {
		status, obs := convergedCluster()
		conditions := deriveConditions(status, obs, PassResult{StoppedBy: "tls", Failure: &ReconcilerError{Reconciler: "tls", Err: errors.New("boom")}, ConsecutiveFailures: 1})
		Expect(apimeta.FindStatusCondition(conditions, opensearchv1.ConditionReady)).To(BeNil())
		expectCondition(conditions, opensearchv1.ConditionProgressing, metav1.ConditionFalse, opensearchv1.ReasonReconciled)
	})

	It("still reports Ready=False from a pass that was cut short", func() {
		status, obs := convergedCluster()
		obs.nodePools[0].readyPods = 2
		conditions := deriveConditions(status, obs, PassResult{StoppedBy: "restart"})
		expectCondition(conditions, opensearchv1.ConditionReady, metav1.ConditionFalse, opensearchv1.ReasonPodsNotReady)
		progressing := expectCondition(conditions, opensearchv1.ConditionProgressing, metav1.ConditionTrue, opensearchv1.ReasonReconciling)
		Expect(progressing.Message).To(Equal("restart reconciler requeued"))
	})

	DescribeTable("reports Degraded",
		func(mutate func(*opensearchv1.ClusterStatus, *clusterObservation), pass PassResult, reason, message string) {
			status, obs := convergedCluster()
			mutate(&status, &obs)
			degraded := expectCondition(deriveConditions(status, obs, pass), opensearchv1.ConditionDegraded, metav1.ConditionTrue, reason)
			Expect(degraded.Message).To(Equal(message))
		},
		Entry("on red health",
			func(_ *opensearchv1.ClusterStatus, o *clusterObservation) {
				o.health = opensearchv1.OpenSearchRedHealth
			},
			PassResult{}, opensearchv1.ReasonClusterRed, "cluster health is red"),
		Entry("on stuck pods, listed in name order",
			func(_ *opensearchv1.ClusterStatus, o *clusterObservation) {
				o.stuckPods = map[string]string{"masters-2": "ImagePullBackOff", "masters-0": "CrashLoopBackOff"}
			},
			PassResult{}, opensearchv1.ReasonStuckPod, "masters-0: CrashLoopBackOff, masters-2: ImagePullBackOff"),
		Entry("on a stalled drain",
			func(s *opensearchv1.ClusterStatus, _ *clusterObservation) {
				s.ComponentsStatus = []opensearchv1.ComponentStatus{{Component: "Scaler", Status: "Excluded", Description: "masters", Conditions: []string{"masters-2", drainStalledCondition}}}
			},
			PassResult{}, opensearchv1.ReasonDrainStalled, "node pool masters: drain of masters-2 has stalled"),
		Entry("on a reconciler failing pass after pass",
			func(*opensearchv1.ClusterStatus, *clusterObservation) {},
			PassResult{StoppedBy: "tls", Failure: &ReconcilerError{Reconciler: "tls", Err: errors.New("secret not found")}, ConsecutiveFailures: DegradedAfterFailedPasses},
			opensearchv1.ReasonReconcileError, "tls reconciler: secret not found"),
	)

	It("does not report Degraded for a reconciler error until it repeats", func() {
		status, obs := convergedCluster()
		pass := PassResult{StoppedBy: "tls", Failure: &ReconcilerError{Reconciler: "tls", Err: errors.New("conflict")}, ConsecutiveFailures: DegradedAfterFailedPasses - 1}
		expectCondition(deriveConditions(status, obs, pass), opensearchv1.ConditionDegraded, metav1.ConditionFalse, opensearchv1.ReasonAsExpected)
	})

	It("keeps the Degraded message while the same reconciler keeps failing", func() {
		status, obs := convergedCluster()
		failing := func(reconciler, err string) PassResult {
			return PassResult{StoppedBy: reconciler, Failure: &ReconcilerError{Reconciler: reconciler, Err: errors.New(err)}, ConsecutiveFailures: DegradedAfterFailedPasses + 1}
		}
		status.Conditions = deriveConditions(status, obs, failing("snapshot_repository", "read tcp 10.0.0.5:41234: connection reset"))

		degraded := expectCondition(deriveConditions(status, obs, failing("snapshot_repository", "read tcp 10.0.0.5:51877: connection reset")),
			opensearchv1.ConditionDegraded, metav1.ConditionTrue, opensearchv1.ReasonReconcileError)
		Expect(degraded.Message).To(Equal("snapshot_repository reconciler: read tcp 10.0.0.5:41234: connection reset"))

		degraded = expectCondition(deriveConditions(status, obs, failing("tls", "secret not found")),
			opensearchv1.ConditionDegraded, metav1.ConditionTrue, opensearchv1.ReasonReconcileError)
		Expect(degraded.Message).To(Equal("tls reconciler: secret not found"))
	})

	It("truncates a reconciler error to the condition message limit", func() {
		status, obs := convergedCluster()
		pass := PassResult{Terminal: &ReconcilerError{Reconciler: "upgrade", Err: errors.New(strings.Repeat("x", 40000))}}
		ready := expectCondition(deriveConditions(status, obs, pass), opensearchv1.ConditionReady, metav1.ConditionFalse, opensearchv1.ReasonReconcileError)
		Expect(ready.Message).To(HaveLen(32768))
	})
})
