package cluster

import (
	"context"
	"fmt"
	sts "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	opsterv1 "os-operator.io/api/v1"
	"os-operator.io/pkg/builders"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	controllerName           = "cluster-controller"
	configHashAnnotationName = "cluster.k8s.elastic.co/config-hash"
)

type State struct {
	Compenent string `json:"compenent,omitempty"`
	Status    string `json:"status,omitempty"`
	Err       error  `json:"err,omitempty"`
}

type ClusterReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	State    State
	Instnce  *opsterv1.Os
}

//+kubebuilder:rbac:groups="opster.os-operator.opster.io",resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=opster.os-operator.opster.io,resources=os,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=opster.os-operator.opster.io,resources=os/status/componenetsStatus,verbs=get;update;patch
//+kubebuilder:rbac:groups=opster.os-operator.opster.io,resources=os/finalizers,verbs=update

func (r *ClusterReconciler) InternalReconcile(ctx context.Context) (ClusterReconciler, ctrl.Result, error) {

	cm := v1.ConfigMap{}
	cluster_reconciler := ClusterReconciler{}
	namesapce := r.Instnce.Spec.General.ClusterName

	cmName := "opensearch-yml"

	if err := r.Get(context.TODO(), client.ObjectKey{Name: cmName, Namespace: namesapce}, &cm); err != nil {

		clusterCm := builders.NewCmForCR(r.Instnce)
		err = r.Create(context.TODO(), clusterCm)
		if err != nil {
			fmt.Println(err, "Cannot create Configmap "+clusterCm.Name)
			cluster_reconciler.State.Status = "Failed"
			return cluster_reconciler, ctrl.Result{}, err
		}
		fmt.Println("Cm Created successfully", "name", clusterCm.Name)
	}

	healessService := v1.Service{}
	serviceName := r.Instnce.Spec.General.ServiceName + "-headleass-service"
	if err := r.Get(context.TODO(), client.ObjectKey{Name: serviceName, Namespace: namesapce}, &healessService); err != nil {

		/// ------ Create Headleass Service -------
		headless_service := builders.NewHeadlessServiceForCR(r.Instnce)

		err = r.Create(context.TODO(), headless_service)
		if err != nil {
			fmt.Println(err, "Cannot create Headless Service")
			cluster_reconciler.State.Status = "Failed"
			return cluster_reconciler, ctrl.Result{}, err
		}
		fmt.Println("service Created successfully", "name", headless_service.Name)
	}

	service := v1.Service{}
	serviceName = r.Instnce.Spec.General.ServiceName + "-svc"
	if err := r.Get(context.TODO(), client.ObjectKey{Name: serviceName, Namespace: namesapce}, &service); err != nil {

		/// ------ Create External Service -------
		clusterService := builders.NewServiceForCR(r.Instnce)

		err = r.Create(context.TODO(), clusterService)
		if err != nil {
			fmt.Println(err, "Cannot create service")
			cluster_reconciler.State.Status = "Failed"
			return cluster_reconciler, ctrl.Result{}, err
		}
		fmt.Println("service Created successfully", "name", service.Name)

	}

	///// ------ Create Es Nodes StatefulSet -------
	NodesCount := len(r.Instnce.Spec.OsNodes)
	sts := sts.StatefulSet{}

	for x := 0; x < NodesCount; x++ {
		sts_for_build := builders.NewSTSForCR(r.Instnce, r.Instnce.Spec.OsNodes[x])
		stsName := r.Instnce.Spec.General.ClusterName + "-" + r.Instnce.Spec.OsNodes[x].Compenent
		if err := r.Get(context.TODO(), client.ObjectKey{Name: stsName, Namespace: namesapce}, &sts); err != nil {
			/// ------ Create Es StatefulSet -------
			fmt.Println("Starting create ", r.Instnce.Spec.OsNodes[x].Compenent, " Sts")
			//	r.StsCreate(ctx, &sts_for_build)
			err := r.Create(context.TODO(), sts_for_build)
			if err != nil {
				cluster_reconciler.State.Status = "Failed"
				return cluster_reconciler, ctrl.Result{}, err
			}
			fmt.Println(r.Instnce.Spec.OsNodes[x].Compenent, " StatefulSet has Created successfully")
		}

	}

	cluster_reconciler.State.Status = "Done"
	return cluster_reconciler, ctrl.Result{}, nil
}
