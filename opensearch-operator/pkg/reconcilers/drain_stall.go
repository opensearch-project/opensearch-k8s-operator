package reconcilers

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/client-go/tools/record"

	opensearchv1 "github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/api/opensearch.org/v1"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/opensearch-gateway/services"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/k8s"
	"github.com/opensearch-project/opensearch-k8s-operator/opensearch-operator/pkg/reconcilers/util"
)

// drainComponentName is the component status the drain state is kept under. Its
// own component rather than the restarter's or the upgrader's, because both of
// them rewrite their conditions from scratch on every pass, and this state has to
// survive between passes to mean anything.
const drainComponentName = "Drain"

// drainStateFor returns the drain conditions recorded for podName, or nil when
// what is recorded belongs to another candidate.
func drainStateFor(instance *opensearchv1.OpenSearchCluster, podName string) []string {
	for _, component := range instance.Status.ComponentsStatus {
		if component.Component != drainComponentName {
			continue
		}
		if len(component.Conditions) == 0 || component.Conditions[0] != podName {
			return nil
		}
		return component.Conditions
	}
	return nil
}

// recordDrainState persists the drain conditions, and writes nothing when they
// have not changed, so a stalled drain does not rewrite the status every ten
// seconds.
func recordDrainState(k8sClient k8s.K8sClient, instance *opensearchv1.OpenSearchCluster, podName string, conditions []string) error {
	for _, component := range instance.Status.ComponentsStatus {
		if component.Component == drainComponentName && drainConditionsEqual(component.Conditions, conditions) {
			return nil
		}
	}
	return UpdateComponentStatus(k8sClient, instance, &opensearchv1.ComponentStatus{
		Component:   drainComponentName,
		Status:      "Draining",
		Description: podName,
		Conditions:  conditions,
	})
}

// drainStallOutcome is what one reconcile pass makes of a drain that has not
// finished yet.
type drainStallOutcome struct {
	// conditions to persist on the component status, carrying the candidate,
	// when its drain started, and whether it is considered stalled.
	conditions []string
	// release is true on the single pass that crosses the threshold, and is the
	// signal to withdraw the allocation exclusion.
	release bool
	// warn rides with release, so an operator is told once rather than every
	// ten seconds.
	warn bool
	// stalled is the state after this pass, kept for the caller's log line.
	stalled bool
}

// evaluateDrainStall folds one pass into the drain's stall state.
//
// activity means the allocator is still moving shards, and it restarts the
// clock: a drain making progress is never stalled, however long it takes. Only
// quiet passes accumulate, and once they have accumulated past the threshold the
// exclusion has bought nothing and is released, once.
//
// A different candidate starts its own clock, because the conditions belong to
// the node being drained rather than to the cluster.
func evaluateDrainStall(current []string, podName string, activity bool, now time.Time, threshold time.Duration) drainStallOutcome {
	if len(current) == 0 || current[0] != podName {
		current = nil
	}

	if activity {
		return drainStallOutcome{conditions: drainConditions(podName, now, false)}
	}

	startedAt, ok := drainStartedAt(current)
	if !ok {
		startedAt = now
	}
	already := hasDrainStalledCondition(current)
	stalled := already || drainHasStalled(startedAt, now, threshold)

	return drainStallOutcome{
		conditions: drainConditions(podName, startedAt, stalled),
		release:    stalled && !already,
		warn:       stalled && !already,
		stalled:    stalled,
	}
}

// standStillOnStalledDrain reports whether the reconciler should leave the
// cluster alone this pass.
//
// Once a drain has been declared stalled its exclusion is gone, and calling
// PreparePodForDelete again would simply re-apply it, which is the flapping this
// whole path exists to avoid. So the candidate is left as it is until something
// actually changes: shards start moving again, or the node turns out to be empty
// after all. Both clear the stall and let the ordinary path resume.
func standStillOnStalledDrain(osClient *services.OsClusterClient, podName string, current []string, now time.Time) (bool, []string, error) {
	if len(current) == 0 || current[0] != podName || !hasDrainStalledCondition(current) {
		return false, nil, nil
	}

	health, err := osClient.GetHealth()
	if err != nil {
		return false, nil, err
	}
	if services.HasShardActivity(health) {
		return false, drainConditions(podName, now, false), nil
	}

	notEmpty, err := services.HasShardsOnNode(osClient, podName)
	if err != nil {
		return false, nil, err
	}
	if !notEmpty {
		return false, nil, nil
	}
	return true, current, nil
}

// recordUnfinishedDrain is called on a pass where the candidate could not be
// deleted with draining on. It keeps the stall clock, and on the pass that
// crosses the threshold it withdraws the exclusion and says so once.
func recordUnfinishedDrain(
	instance *opensearchv1.OpenSearchCluster,
	osClient *services.OsClusterClient,
	recorder record.EventRecorder,
	lg logr.Logger,
	podName string,
	current []string,
	now time.Time,
) ([]string, error) {
	health, err := osClient.GetHealth()
	if err != nil {
		return nil, err
	}
	outcome := evaluateDrainStall(current, podName, services.HasShardActivity(health), now, drainStallWarningAfter)
	if !outcome.release {
		return outcome.conditions, nil
	}

	released, err := util.ReleaseDrainExclusion(instance, osClient, lg, podName)
	if err != nil {
		return nil, err
	}
	blocking := blockingShard(osClient, podName)
	lg.Info(fmt.Sprintf(
		"Drain of %s has made no progress for %s and its exclusion has been released%s; the restart stays blocked until the shards it holds can move",
		podName, drainStallWarningAfter, blocking), "exclusionReleased", released)
	if recorder != nil {
		recorder.AnnotatedEventf(instance, map[string]string{"cluster-name": instance.GetName()}, "Warning", "RollingRestart",
			"Drain of node %s has made no progress for %s%s; its allocation exclusion was released, and the restart waits until those shards can move. Check remaining capacity, disk watermarks, allocation filters or replica settings",
			podName, drainStallWarningAfter, blocking)
	}
	return outcome.conditions, nil
}

// blockingShard names one shard still on the node, for the message. Best effort:
// the message is worth having without it, so a failure here is not worth failing
// the reconcile over.
func blockingShard(osClient *services.OsClusterClient, podName string) string {
	shards, err := osClient.CatShards([]string{"index", "shard", "prirep", "state", "node"})
	if err != nil {
		return ""
	}
	for _, shard := range shards {
		// A relocating shard's node column reads "source -> ip id target", so the
		// first field is the node it is still on.
		fields := strings.Fields(shard.NodeName)
		if len(fields) > 0 && fields[0] == podName {
			return fmt.Sprintf(", starting with %s[%s]", shard.Index, shard.Shard)
		}
	}
	return ""
}
