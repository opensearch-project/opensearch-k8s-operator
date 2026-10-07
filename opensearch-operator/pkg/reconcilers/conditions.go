package reconcilers

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"

	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/builders"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/helpers"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/util"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// DegradedAfterFailedPasses is how many passes in a row must end in a non-terminal
// reconciler error before Degraded reports it, so a one-off conflict does not flap it.
const DegradedAfterFailedPasses = 3

// ReconcilerError is an error returned by one reconciler of the chain.
type ReconcilerError struct {
	Reconciler string
	Err        error
}

// PassResult is how the reconciler chain ended in one pass.
type PassResult struct {
	// StoppedBy is the reconciler that ended the chain early with a requeue or a
	// non-terminal error; empty when every reconciler ran.
	StoppedBy string
	// Failure is the non-terminal error that ended the chain, if any.
	Failure *ReconcilerError
	// ConsecutiveFailures counts passes in a row that ended with a Failure, this one included.
	ConsecutiveFailures int
	// Terminal is the first terminal error; the chain kept going past it.
	Terminal *ReconcilerError
}

type nodePoolObservation struct {
	component string
	desired   int32
	sts       *appsv1.StatefulSet // nil until the cluster reconciler creates it
	readyPods int32
}

type clusterObservation struct {
	health    opensearchv1.OpenSearchHealth
	nodePools []nodePoolObservation
	stuckPods map[string]string
}

// UpdateClusterConditions writes health, available nodes, observedGeneration and the
// Ready/Progressing/Degraded conditions. It runs after the reconciler chain on every
// pass, whether or not a reconciler failed, so these never go stale.
func UpdateClusterConditions(
	c client.Client,
	ctx context.Context,
	instance *opensearchv1.OpenSearchCluster,
	transport http.RoundTripper,
	pass PassResult,
) error {
	lg := log.FromContext(ctx)
	k8sClient := k8s.NewK8sClient(c, ctx)

	health, healthResponse := util.GetClusterHealth(k8sClient, ctx, instance, transport, lg)
	availableNodes := util.GetAvailableOpenSearchNodes(k8sClient, ctx, instance, lg)
	helpers.UpdateClusterInfo(instance, health, healthResponse)

	obs := clusterObservation{health: health, stuckPods: map[string]string{}}
	for i := range instance.Spec.NodePools {
		nodePool := &instance.Spec.NodePools[i]
		pool := nodePoolObservation{component: nodePool.Component, desired: nodePool.Replicas}
		sts, err := k8sClient.GetStatefulSet(builders.StsName(instance, nodePool), instance.Namespace)
		if err != nil && !k8serrors.IsNotFound(err) {
			return err
		}
		if err == nil {
			pool.sts = &sts
		}
		pool.readyPods, err = helpers.ReadyReplicasForNodePool(k8sClient, instance, nodePool)
		if err != nil {
			return err
		}
		pods, err := helpers.ListPodsForNodePool(k8sClient, instance, nodePool)
		if err != nil {
			return err
		}
		for j := range pods {
			if reason := helpers.StuckContainerReason(&pods[j]); reason != "" {
				obs.stuckPods[pods[j].Name] = reason
			}
		}
		obs.nodePools = append(obs.nodePools, pool)
	}

	apply := func(cluster *opensearchv1.OpenSearchCluster) {
		cluster.Status.Health = health
		cluster.Status.AvailableNodes = availableNodes
		cluster.Status.ObservedGeneration = instance.Generation
		for _, condition := range deriveConditions(cluster.Status, obs, pass) {
			condition.ObservedGeneration = instance.Generation
			apimeta.SetStatusCondition(&cluster.Status.Conditions, condition)
		}
	}

	current, err := k8sClient.GetOpenSearchCluster(instance.Name, instance.Namespace)
	if err != nil {
		return err
	}
	desired := current.DeepCopy()
	apply(desired)
	if equality.Semantic.DeepEqual(current.Status, desired.Status) {
		return nil
	}
	return k8sClient.UpdateOpenSearchClusterStatus(client.ObjectKeyFromObject(instance), apply)
}

// deriveConditions returns the conditions to set. Ready is left out when the pass was cut
// short and would otherwise report True: a later reconciler may not have acted on the spec
// yet, so the previous Ready (and its observedGeneration) stays in place.
func deriveConditions(status opensearchv1.ClusterStatus, obs clusterObservation, pass PassResult) []metav1.Condition {
	conditions := []metav1.Condition{progressingCondition(status, obs, pass), degradedCondition(status, obs, pass)}
	ready := readyCondition(status, obs, pass)
	if ready.Status == metav1.ConditionTrue && pass.StoppedBy != "" {
		return conditions
	}
	return append(conditions, ready)
}

func readyCondition(status opensearchv1.ClusterStatus, obs clusterObservation, pass PassResult) metav1.Condition {
	notReady := func(reason, message string) metav1.Condition {
		return metav1.Condition{Type: opensearchv1.ConditionReady, Status: metav1.ConditionFalse, Reason: reason, Message: message}
	}
	if !status.Initialized {
		return notReady(opensearchv1.ReasonInitializing, "waiting for the cluster to bootstrap")
	}
	if work, ok := workInFlight(status, obs); ok {
		return notReady(work.readyReason, work.message)
	}
	if pass.Terminal != nil {
		return notReady(opensearchv1.ReasonReconcileError, reconcilerErrorMessage(pass.Terminal))
	}
	if obs.health == "" || obs.health == opensearchv1.OpenSearchUnknownHealth {
		return notReady(opensearchv1.ReasonClusterUnreachable, "cluster health could not be read")
	}
	for _, pool := range obs.nodePools {
		if message := nodePoolNotReady(pool); message != "" {
			return notReady(opensearchv1.ReasonPodsNotReady, message)
		}
	}
	return metav1.Condition{Type: opensearchv1.ConditionReady, Status: metav1.ConditionTrue, Reason: opensearchv1.ReasonReconciled, Message: "all node pools are ready"}
}

func progressingCondition(status opensearchv1.ClusterStatus, obs clusterObservation, pass PassResult) metav1.Condition {
	progressing := func(reason, message string) metav1.Condition {
		return metav1.Condition{Type: opensearchv1.ConditionProgressing, Status: metav1.ConditionTrue, Reason: reason, Message: message}
	}
	if !status.Initialized {
		return progressing(opensearchv1.ReasonInitializing, "waiting for the cluster to bootstrap")
	}
	if work, ok := workInFlight(status, obs); ok {
		return progressing(work.component, work.message)
	}
	if pass.StoppedBy != "" && pass.Failure == nil {
		return progressing(opensearchv1.ReasonReconciling, fmt.Sprintf("%s reconciler requeued", pass.StoppedBy))
	}
	return metav1.Condition{Type: opensearchv1.ConditionProgressing, Status: metav1.ConditionFalse, Reason: opensearchv1.ReasonReconciled, Message: "no work in flight"}
}

func degradedCondition(status opensearchv1.ClusterStatus, obs clusterObservation, pass PassResult) metav1.Condition {
	degraded := func(reason, message string) metav1.Condition {
		return metav1.Condition{Type: opensearchv1.ConditionDegraded, Status: metav1.ConditionTrue, Reason: reason, Message: message}
	}
	if obs.health == opensearchv1.OpenSearchRedHealth {
		return degraded(opensearchv1.ReasonClusterRed, "cluster health is red")
	}
	if len(obs.stuckPods) > 0 {
		pods := make([]string, 0, len(obs.stuckPods))
		for pod, reason := range obs.stuckPods {
			pods = append(pods, fmt.Sprintf("%s: %s", pod, reason))
		}
		sort.Strings(pods)
		return degraded(opensearchv1.ReasonStuckPod, strings.Join(pods, ", "))
	}
	for _, cs := range status.ComponentsStatus {
		if cs.Component == "Scaler" && hasDrainStalledCondition(cs.Conditions) {
			return degraded(opensearchv1.ReasonDrainStalled, fmt.Sprintf("node pool %s: drain of %s has stalled", cs.Description, scalerTargetNodeName(cs.Conditions)))
		}
	}
	if pass.Failure != nil && pass.ConsecutiveFailures >= DegradedAfterFailedPasses {
		return degraded(opensearchv1.ReasonReconcileError, reconcilerErrorMessage(pass.Failure))
	}
	return metav1.Condition{Type: opensearchv1.ConditionDegraded, Status: metav1.ConditionFalse, Reason: opensearchv1.ReasonAsExpected, Message: ""}
}

type inFlightWork struct {
	component   string
	readyReason string
	message     string
}

// workInFlight returns the first sub-reconciler operation still running, in the order
// they block Ready.
func workInFlight(status opensearchv1.ClusterStatus, obs clusterObservation) (inFlightWork, bool) {
	if cs, ok := findComponent(status, securityConfigComponentName); ok &&
		(cs.Status == securityConfigStatusRunning || cs.Status == securityConfigStatusFailed) {
		return inFlightWork{securityConfigComponentName, opensearchv1.ReasonSecurityConfigPending, describeComponent(cs)}, true
	}
	if message, ok := upgradeInFlight(status); ok {
		return inFlightWork{componentNameUpgrader, opensearchv1.ReasonUpgrading, message}, true
	}
	if cs, ok := findComponent(status, "Scaler"); ok {
		return inFlightWork{"Scaler", opensearchv1.ReasonScaling, describeComponent(cs)}, true
	}
	for _, pool := range obs.nodePools {
		if pool.sts != nil && ptr.Deref(pool.sts.Spec.Replicas, 1) != pool.desired {
			return inFlightWork{"Scaler", opensearchv1.ReasonScaling, fmt.Sprintf("node pool %s: %d of %d replicas", pool.component, ptr.Deref(pool.sts.Spec.Replicas, 1), pool.desired)}, true
		}
	}
	if cs, ok := findComponent(status, componentName); ok && cs.Status == statusInProgress {
		return inFlightWork{componentName, opensearchv1.ReasonRollingRestart, "rolling restart in progress"}, true
	}
	return inFlightWork{}, false
}

func upgradeInFlight(status opensearchv1.ClusterStatus) (string, bool) {
	target, pool := "", ""
	found := false
	for _, cs := range status.ComponentsStatus {
		if cs.Component != componentNameUpgrader {
			continue
		}
		found = true
		switch {
		case cs.Description == upgradeTargetDescription:
			target = cs.Status
		case cs.Status == upgradeStatusInProgress:
			pool = cs.Description
		}
	}
	if !found {
		return "", false
	}
	if pool != "" {
		return fmt.Sprintf("upgrading node pool %s to %s", pool, target), true
	}
	return fmt.Sprintf("upgrading to %s", target), true
}

// nodePoolNotReady returns why the pool's StatefulSet has not converged, or "" if it has.
func nodePoolNotReady(pool nodePoolObservation) string {
	if pool.sts == nil {
		return fmt.Sprintf("node pool %s: StatefulSet not created yet", pool.component)
	}
	replicas := ptr.Deref(pool.sts.Spec.Replicas, 1)
	if pool.sts.Status.ObservedGeneration < pool.sts.Generation {
		return fmt.Sprintf("node pool %s: StatefulSet update not observed yet", pool.component)
	}
	// Same check as the rolling restart reconciler: UpdateRevision is empty until the
	// StatefulSet controller has rolled out a revision.
	if pool.sts.Status.UpdateRevision != "" && pool.sts.Status.UpdatedReplicas != replicas {
		return fmt.Sprintf("node pool %s: %d/%d pods updated", pool.component, pool.sts.Status.UpdatedReplicas, replicas)
	}
	if pool.readyPods != replicas {
		return fmt.Sprintf("node pool %s: %d/%d pods ready", pool.component, pool.readyPods, replicas)
	}
	return ""
}

func findComponent(status opensearchv1.ClusterStatus, component string) (opensearchv1.ComponentStatus, bool) {
	return helpers.FindFirstPartial(status.ComponentsStatus, opensearchv1.ComponentStatus{Component: component}, helpers.GetByComponent)
}

func describeComponent(cs opensearchv1.ComponentStatus) string {
	message := cs.Status
	if cs.Description != "" {
		message = cs.Description + ": " + message
	}
	if len(cs.Conditions) > 0 {
		message += " (" + strings.Join(cs.Conditions, ", ") + ")"
	}
	return message
}

// maxConditionMessage is metav1.Condition's MaxLength on message; a longer one fails the whole status update.
const maxConditionMessage = 32768

func reconcilerErrorMessage(e *ReconcilerError) string {
	message := fmt.Sprintf("%s reconciler: %v", e.Reconciler, e.Err)
	if len(message) > maxConditionMessage {
		message = message[:maxConditionMessage]
	}
	return message
}
