package controllers

import (
	"context"
	"fmt"

	opsterv1 "../../opensearch-operator/api/v1"
	"../../opensearch-operator/pkg/builders"
	sts "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type State struct {
	Component string `json:"component,omitempty"`
	Status    string `json:"status,omitempty"`
	Err       error  `json:"err,omitempty"`
}

type ClusterReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	State    State
	Instance *opsterv1.OpenSearchCluster
}

//+kubebuilder:rbac:groups="opensearch.opster.io",resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchcluster,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchcluster/status/componentsStatus,verbs=get;update;patch
//+kubebuilder:rbac:groups=opensearch.opster.io,resources=opensearchcluster/finalizers,verbs=update

func (r *ClusterReconciler) Reconcile(context.Context, ctrl.Request) (ctrl.Result, error) {

	cm := v1.ConfigMap{}
	namespace := r.Instance.Spec.General.ClusterName

	cmName := "opensearch-yml"

	if err := r.Get(context.TODO(), client.ObjectKey{Name: cmName, Namespace: namespace}, &cm); err != nil {
		clusterCm := builders.NewCmForCR(r.Instance)
		err = r.Create(context.TODO(), clusterCm)
		if err != nil {
			if !errors.IsAlreadyExists(err) {
				fmt.Println(err, "Cannot create Configmap "+clusterCm.Name)
				return ctrl.Result{}, err
			}
		}
		fmt.Println("Cm Created successfully", "name", clusterCm.Name)
	}

	headlessService := v1.Service{}
	serviceName := r.Instance.Spec.General.ServiceName + "-headless-service"
	if err := r.Get(context.TODO(), client.ObjectKey{Name: serviceName, Namespace: namespace}, &headlessService); err != nil {
		/// ------ Create Headless Service -------
		headless_service := builders.NewHeadlessServiceForCR(r.Instance)

		err = r.Create(context.TODO(), headless_service)
		if err != nil {
			if !errors.IsAlreadyExists(err) {
				fmt.Println(err, "Cannot create Headless Service")
				return ctrl.Result{}, err
			}
		}
		fmt.Println("service Created successfully", "name", headless_service.Name)
	}

	service := v1.Service{}
	serviceName = r.Instance.Spec.General.ServiceName + "-svc"
	if err := r.Get(context.TODO(), client.ObjectKey{Name: serviceName, Namespace: namespace}, &service); err != nil {

		/// ------ Create External Service -------
		clusterService := builders.NewServiceForCR(r.Instance)

		err = r.Create(context.TODO(), clusterService)
		if err != nil {
			if !errors.IsAlreadyExists(err) {
				fmt.Println(err, "Cannot create service")
				return ctrl.Result{}, err
			}

		}
		fmt.Println("service Created successfully", "name", service.Name)

	}

	///// ------ Create Es Nodes StatefulSet -------
	NodesCount := len(r.Instance.Spec.NodePools)
	sts := sts.StatefulSet{}

	for x := 0; x < NodesCount; x++ {
		sts_for_build := builders.NewSTSForCR(r.Instance, r.Instance.Spec.NodePools[x])
		stsName := r.Instance.Spec.General.ClusterName + "-" + r.Instance.Spec.NodePools[x].Component
		if err := r.Get(context.TODO(), client.ObjectKey{Name: stsName, Namespace: namespace}, &sts); err != nil {
			/// ------ Create Es StatefulSet -------
			fmt.Println("Starting create ", r.Instance.Spec.NodePools[x].Component, " Sts")
			//	r.StsCreate(ctx, &sts_for_build)
			err := r.Create(context.TODO(), sts_for_build)
			if err != nil {
				if !errors.IsAlreadyExists(err) {
					fmt.Println(err, "Cannot create-"+stsName+" node group")
					return ctrl.Result{}, err
				}
			}
			fmt.Println(r.Instance.Spec.NodePools[x].Component, " StatefulSet has Created successfully"+"-"+stsName)
		}

	}
	return ctrl.Result{}, nil
}

func (r *ClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := opsterv1.AddToScheme(mgr.GetScheme()); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		For(&opsterv1.OpenSearchCluster{}).
		Complete(r)
}
