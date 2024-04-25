package common

import (
	"context"
	ctrl "sigs.k8s.io/controller-runtime"
)

type ClusterReconcileRequirement interface {
	ClusterPreReconcile
	ClusterRegistry
}

type ClusterRegistry interface {
	RegisterRoles() []RoleReconciler
	RegisterResources() []ResourceReconciler
}

type DiscoveryReconciler interface {
	ReconcileDiscovery() (ctrl.Result, error)
}

type ClusterPreReconcile interface {
	PreReconcile()
}

type ClusterReconciler struct {
	ClusterReconcileRequirement
	DiscoveryReconciler
}

func NewClusterReconciler(requirements ClusterReconcileRequirement) *ClusterReconciler {
	return &ClusterReconciler{
		ClusterReconcileRequirement: requirements,
	}
}

// SetDiscoveryReconciler  set DiscoveryReconcile
func (c *ClusterReconciler) SetDiscoveryReconciler(reconciler DiscoveryReconciler) *ClusterReconciler {
	c.DiscoveryReconciler = reconciler
	return c
}

func (c *ClusterReconciler) ReconcileCluster(ctx context.Context) (ctrl.Result, error) {
	// pre-reconcile
	c.ClusterReconcileRequirement.PreReconcile()

	// reconcile resource of cluster level
	if resources := c.ClusterReconcileRequirement.RegisterResources(); len(resources) > 0 {
		res, err := ReconcilerDoHandler(ctx, resources)
		if err != nil {
			return ctrl.Result{}, err
		}
		if res.RequeueAfter > 0 {
			return res, nil
		}
	}

	//reconcile role
	for _, r := range c.ClusterReconcileRequirement.RegisterRoles() {
		res, err := r.ReconcileRole(ctx)
		if err != nil {
			return ctrl.Result{}, err
		}
		if res.RequeueAfter > 0 {
			return res, nil
		}
	}

	// reconcile discovery
	if c.DiscoveryReconciler != nil {
		res, err := c.DiscoveryReconciler.ReconcileDiscovery()
		if err != nil {
			return ctrl.Result{}, err
		}
		if res.RequeueAfter > 0 {
			return res, nil
		}
	}
	return ctrl.Result{}, nil
}
