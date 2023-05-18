package reconcilers

import (
	"context"
	"encoding/json"
	"github.com/banzaicloud/k8s-objectmatcher/patch"
	"github.com/banzaicloud/operator-tools/pkg/reconciler"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/record"
	opsterv1 "opensearch.opster.io/api/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
	"time"
)

type UpgradeCheckerReconciler struct {
	client.Client
	reconciler.ResourceReconciler
	ctx               context.Context
	recorder          record.EventRecorder
	reconcilerContext *ReconcilerContext
	instance          *opsterv1.OpenSearchCluster
	logger            logr.Logger
}

func NewUpgradeCheckerReconciler(
	client client.Client,
	ctx context.Context,
	recorder record.EventRecorder,
	reconcilerContext *ReconcilerContext,
	instance *opsterv1.OpenSearchCluster,
	opts ...reconciler.ResourceReconcilerOption,
) *UpgradeCheckerReconciler {
	return &UpgradeCheckerReconciler{
		Client: client,
		ResourceReconciler: reconciler.NewReconcilerWith(client,
			append(
				opts,
				reconciler.WithPatchCalculateOptions(patch.IgnoreVolumeClaimTemplateTypeMetaAndStatus(), patch.IgnoreStatusFields()),
				reconciler.WithLog(log.FromContext(ctx).WithValues("reconciler", "UpgradeChecker")),
			)...),
		ctx:               ctx,
		recorder:          recorder,
		reconcilerContext: reconcilerContext,
		instance:          instance,
		logger:            log.FromContext(ctx),
	}
}

type Payload struct {
	UDI                string   `json:"udi"`
	OperatorVersion    string   `json:"operatorVersion"`
	ClusterCount       int      `json:"clusterCount"`
	OsClustersVersions []string `json:"osClustersVersions"`
}

func (r *UpgradeCheckerReconciler) Reconcile() (ctrl.Result, error) {
	requeue := false
	var err error
	var json string
	results := reconciler.CombinedResult{}
	if !isTimeToRunFunction() {
		results.Combine(&ctrl.Result{Requeue: requeue}, nil)
		return results.Result, nil
	}
	json, err = r.BuildJSONPayload()
	if err != nil {
		results.Combine(&ctrl.Result{Requeue: requeue}, err)
		return results.Result, results.Err
	}
	print(json)
	results.Combine(&ctrl.Result{Requeue: requeue}, nil)
	return results.Result, results.Err

}

func isTimeToRunFunction() bool {
	now := time.Now()
	return now.Hour() == 12 && now.Minute() == 0 && now.Second() == 0
}

func (r *UpgradeCheckerReconciler) BuildJSONPayload() (string, error) {
	var versions []string
	var ClusterCount int
	myUid, operatorNamespace, err := r.FindUidFromSecret(r.ctx, r.Client)
	if err != nil {
		return "", err
	}

	OperatorVersion, err := FindOperatorVersion(r.ctx, r.Client, operatorNamespace, r.instance)
	if err != nil {
		return "", err
	}

	ClusterCount, versions, err = FindCountOfOsClusterAndVersions(r.ctx, r.Client)
	if err != nil {
		return "", err
	}
	Pay := Payload{
		UDI:                myUid,
		OperatorVersion:    OperatorVersion,
		ClusterCount:       ClusterCount,
		OsClustersVersions: versions,
	}

	jsonData, err := ConvertToJSON(Pay)
	if err != nil {
		return jsonData, err
	}
	return jsonData, nil

}

func ConvertToJSON(pay Payload) (string, error) {
	jsonData, err := json.Marshal(pay)
	if err != nil {
		return "", err
	}
	return string(jsonData), nil
}

func (r *UpgradeCheckerReconciler) FindUidFromSecret(ctx context.Context, k8sClient client.Client) (string, string, error) {

	secretList := &v1.SecretList{}
	var valueStr string
	var namespace string
	if err := r.List(ctx, secretList); err != nil {
		r.logger.Error(err, "Cannot find UDI secret")
		return "-1", "-1", err
		// Handle the error
	}

	for _, secret := range secretList.Items {
		if secret.Name == "operator-uid" {
			value, ok := secret.Data["secretKey"]
			if !ok {
				r.logger.Info("Cannot secretKey inside of UDI secret")
			}
			valueStr = string(value)
			namespace = secret.Namespace
			r.logger.Info("UID:", valueStr)
			break
		}
	}

	return valueStr, namespace, nil
}

func (r *UpgradeCheckerReconciler) FindOperatorVersion(ctx context.Context, k8sClient client.Client, operatorNamespace string) (string, error) {
	deployOperator := &appsv1.Deployment{}
	var imageVersion string
	err := k8sClient.Get(ctx, client.ObjectKey{Name: "opensearch-operator-controller-manager", Namespace: operatorNamespace}, deployOperator)
	if err != nil {
		r.logger.Error(err, "Cannot find Operator Deployment")
		return "0", err
	}

	for i := 0; i < len(deployOperator.Spec.Template.Spec.Containers); i++ {
		imageVersion = deployOperator.Spec.Template.Spec.Containers[i].Image
		if strings.Contains(imageVersion, "opensearch-operator") {
			break
		}

	}

	version := findVersion(imageVersion)
	return version, err

}

func (r *UpgradeCheckerReconciler) FindCountOfOsClusterAndVersions(ctx context.Context, k8sClient client.Client) (int, []string, error) {
	var empty []string
	list := &opsterv1.OpenSearchClusterList{}
	if err := k8sClient.List(ctx, list); err != nil {
		r.logger.Error(err, "Cannot find the CRD instances ")
		return 0, empty, err
	}
	var clustersVersion []string
	for cluster := 0; cluster < len(list.Items); cluster++ {
		clustersVersion = append(clustersVersion, list.Items[cluster].Spec.General.Version)
	}

	return len(list.Items), clustersVersion, nil
}

func findVersion(image string) string {
	index := strings.Index(image, ":")
	ver := image[index+1:]
	return ver
}
