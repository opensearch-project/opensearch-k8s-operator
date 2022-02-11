package controllers

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	opsterv1 "opensearch.opster.io/api/v1"
	"opensearch.opster.io/pkg/builders"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ClusterReconciler struct {
	client.Client
	Recorder record.EventRecorder
	Instance *opsterv1.OpenSearchCluster
}

func (r *ClusterReconciler) Reconcile(controllerContext *ControllerContext) (*opsterv1.ComponentStatus, error) {

	namespace := r.Instance.Spec.General.ClusterName

	service := v1.Service{}
	serviceName := r.Instance.Spec.General.ServiceName
	if err := r.Get(context.TODO(), client.ObjectKey{Name: serviceName, Namespace: namespace}, &service); err != nil {
		// Create External Service
		clusterService := builders.NewServiceForCR(r.Instance)

		err = r.Create(context.TODO(), clusterService)
		if err != nil {
			if !errors.IsAlreadyExists(err) {
				fmt.Println(err, "Cannot create service")
				r.Recorder.Event(r.Instance, "Warning", "Cannot create opensearch Service ", "Requeue - Fix the problem you have on main Opensearc Service ")
				return nil, err
			}

		}
		fmt.Println("service Created successfully", "name", service.Name)
	}

	// Create StatefulSets for NodePools
	for _, nodePool := range r.Instance.Spec.NodePools {
		// Create headless service for sts
		targetService := builders.NewHeadlessServiceForNodePool(r.Instance, &nodePool)
		existingService := v1.Service{}
		if err := r.Get(context.TODO(), client.ObjectKey{Name: targetService.Name, Namespace: namespace}, &existingService); err != nil {
			err = r.Create(context.TODO(), targetService)
			if err != nil {
				if !errors.IsAlreadyExists(err) {
					fmt.Println(err, "Cannot create Headless Service")
					r.Recorder.Event(r.Instance, "Warning", "Cannot create Headless Service ", "Requeue - Fix the problem you have on main Opensearch Headless Service ")
					return nil, err
				}
			}
			fmt.Println("service Created successfully", "name", targetService.Name)
		}

		stsName := r.Instance.Spec.General.ClusterName + "-" + nodePool.Component
		targetSTS := builders.NewSTSForNodePool(r.Instance, nodePool, controllerContext.Volumes, controllerContext.VolumeMounts)
		existingSTS := appsv1.StatefulSet{}
		if err := r.Get(context.TODO(), client.ObjectKey{Name: stsName, Namespace: namespace}, &existingSTS); err != nil {
			fmt.Printf("Creating statefulset for nodepool %s\n", nodePool.Component)
			err = r.Create(context.TODO(), &targetSTS)
			if err != nil {
				if !errors.IsAlreadyExists(err) {
					fmt.Println(err, "Cannot create-"+stsName+" node group")
					r.Recorder.Event(r.Instance, "Warning", "Cannot create Opensearch node group (StateFulSet) ", "Requeue - Fix the problem you have on one of Opensearch NodePools")
					return nil, err
				}
			}
			fmt.Println(nodePool.Component, " StatefulSet has Created successfully"+"-"+stsName)
		}
	}

	return nil, nil
}
