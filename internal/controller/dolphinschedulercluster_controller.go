/*
Copyright 2024 zncdatadev.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/util/retry"

	dolphinschedulerv1alpha1 "github.com/zncdatadev/dolphinscheduler-operator/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DolphinschedulerClusterReconciler reconciles a DolphinschedulerCluster object
type DolphinschedulerClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

//+kubebuilder:rbac:groups=dolphinscheduler.zncdata.dev,resources=dolphinschedulerclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dolphinscheduler.zncdata.dev,resources=dolphinschedulerclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dolphinscheduler.zncdata.dev,resources=dolphinschedulerclusters/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the DolphinschedulerCluster object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *DolphinschedulerClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log.Info("Reconciling DolphionScheduler cluster instance")

	cr := &dolphinschedulerv1alpha1.DolphinschedulerCluster{}

	if err := r.Get(ctx, req.NamespacedName, cr); err != nil {
		if client.IgnoreNotFound(err) != nil {
			r.Log.Error(err, "unable to fetch DolphionSchedulerCluster")
			return ctrl.Result{}, err
		}
		r.Log.Info("DolphionScheduler-cluster resource not found. Ignoring since object must be deleted")
		return ctrl.Result{}, nil
	}

	r.Log.Info("DolphionSchedulerCluster found", "Name", cr.Name)
	// reconcile order by "cluster -> role -> role-group -> resource"
	result, err := NewDolphinSchedulerCluster(r.Scheme, cr, r.Client).ReconcileCluster(ctx)
	if err != nil {
		return ctrl.Result{}, err
	} else if result.RequeueAfter > 0 {
		return result, nil
	}
	r.Log.Info("Reconcile successfully ", "Name", cr.Name)
	return ctrl.Result{}, nil
}

// UpdateStatus updates the status of the DolphionSchedulerCluster resource
// https://stackoverflow.com/questions/76388004/k8s-controller-update-status-and-condition
func (r *DolphinschedulerClusterReconciler) UpdateStatus(ctx context.Context, instance *dolphinschedulerv1alpha1.DolphinschedulerCluster) error {
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return r.Status().Update(ctx, instance)
		//return r.Status().Patch(ctx, instance, client.MergeFrom(instance))
	})

	if retryErr != nil {
		r.Log.Error(retryErr, "Failed to update vfm status after retries")
		return retryErr
	}

	r.Log.V(1).Info("Successfully patched object status")
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DolphinschedulerClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dolphinschedulerv1alpha1.DolphinschedulerCluster{}).
		Owns(&appsv1.StatefulSet{}).
		Complete(r)
}
