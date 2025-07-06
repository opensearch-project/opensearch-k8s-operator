package conditions

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	opsterv1 "github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1"
)

// Condition types used by the operator.
const (
	ConditionReady       = "Ready"
	ConditionReconciling = "Reconciling"
)

// Set sets or updates a condition in the status slice and refreshes ObservedGeneration.
func Set(cluster *opsterv1.OpenSearchCluster, cond metav1.Condition) {
	cond.LastTransitionTime = metav1.Now()
	meta.SetStatusCondition(&cluster.Status.Conditions, cond)
	// Always update ObservedGeneration when we touch conditions so users can see
	// the controller processed this generation.
	cluster.Status.ObservedGeneration = cluster.GetGeneration()
}

// SetReady marks the Ready condition as True/False with a reason & message.
func SetReady(cluster *opsterv1.OpenSearchCluster, ready bool, reason, message string) {
	status := metav1.ConditionFalse
	if ready {
		status = metav1.ConditionTrue
	}
	Set(cluster, metav1.Condition{
		Type:    ConditionReady,
		Status:  status,
		Reason:  reason,
		Message: message,
	})
}

// SetReconciling marks the Reconciling condition.
func SetReconciling(cluster *opsterv1.OpenSearchCluster, inProgress bool, reason, message string) {
	status := metav1.ConditionFalse
	if inProgress {
		status = metav1.ConditionTrue
	}
	Set(cluster, metav1.Condition{
		Type:    ConditionReconciling,
		Status:  status,
		Reason:  reason,
		Message: message,
	})
}
