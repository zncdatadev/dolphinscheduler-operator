package controller

import (
	dolphinv1alpha1 "github.com/zncdata-labs/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdata-labs/dolphinscheduler-operator/internal/common"
	"github.com/zncdata-labs/dolphinscheduler-operator/internal/controller/alerter"
	"github.com/zncdata-labs/dolphinscheduler-operator/internal/controller/api"
	"github.com/zncdata-labs/dolphinscheduler-operator/internal/controller/master"
	"github.com/zncdata-labs/dolphinscheduler-operator/internal/controller/worker"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewDolphinSchedulerCluster(
	scheme *runtime.Scheme,
	instance *dolphinv1alpha1.DolphinschedulerCluster,
	client client.Client) *common.ClusterReconciler {
	return common.NewClusterReconciler(NewDolphinSchedulerClusterReconcileRequirement(scheme, instance, client))
}

var _ common.ClusterReconcileRequirement = &DolphinSchedulerClusterReconcileRequirement{}

type DolphinSchedulerClusterReconcileRequirement struct {
	scheme   *runtime.Scheme
	instance *dolphinv1alpha1.DolphinschedulerCluster
	client   client.Client

	roles     []common.RoleReconciler
	resources []common.ResourceReconciler
}

func NewDolphinSchedulerClusterReconcileRequirement(
	scheme *runtime.Scheme,
	instance *dolphinv1alpha1.DolphinschedulerCluster,
	client client.Client,
) *DolphinSchedulerClusterReconcileRequirement {
	return &DolphinSchedulerClusterReconcileRequirement{
		scheme:   scheme,
		instance: instance,
		client:   client,
	}
}

func (d *DolphinSchedulerClusterReconcileRequirement) RegisterRoles() []common.RoleReconciler {
	if len(d.roles) != 0 {
		return d.roles
	}
	roles := make([]common.RoleReconciler, 0)
	roles = append(roles, master.NewMasterRole(d.scheme, d.instance, d.client))   // register master role
	roles = append(roles, worker.NewWorkerRole(d.scheme, d.instance, d.client))   // register master role
	roles = append(roles, api.NewApiRole(d.scheme, d.instance, d.client))         // register api role
	roles = append(roles, alerter.NewAlerterRole(d.scheme, d.instance, d.client)) // register alerter role
	return roles
}

func (d *DolphinSchedulerClusterReconcileRequirement) RegisterResources() []common.ResourceReconciler {
	if len(d.resources) != 0 {
		return d.resources
	}
	lables := d.instance.Labels
	resources := make([]common.ResourceReconciler, 0)
	resources = append(resources, NewJobInitScriptReconciler(d.scheme, d.instance, d.client, lables, nil))
	resources = append(resources, NewServiceAccount(d.scheme, d.instance, d.client, lables, nil))
	return resources
}

func (d *DolphinSchedulerClusterReconcileRequirement) PreReconcile() {
	roles := d.RegisterRoles()
	for _, role := range roles {
		role.MergeConfig()
	}
}
