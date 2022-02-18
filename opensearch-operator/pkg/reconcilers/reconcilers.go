package reconcilers

import "sigs.k8s.io/controller-runtime/pkg/reconcile"

type ComponentReconciler func() (reconcile.Result, error)
