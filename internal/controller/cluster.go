package controller

import (
	dolphinv1alpha1 "github.com/zncdata-labs/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdata-labs/dolphinscheduler-operator/internal/controller/alerter"
	"github.com/zncdata-labs/dolphinscheduler-operator/internal/controller/api"
	"github.com/zncdata-labs/dolphinscheduler-operator/internal/controller/master"
	"github.com/zncdata-labs/dolphinscheduler-operator/internal/controller/worker"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/core"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewDolphinSchedulerCluster(
	scheme *runtime.Scheme,
	instance *dolphinv1alpha1.DolphinschedulerCluster,
	client client.Client) *core.ClusterReconciler {
	return core.NewClusterReconciler(NewDolphinSchedulerClusterReconcileRequirement(scheme, instance, client))
}

var _ core.ClusterReconcileRequirement = &DolphinSchedulerClusterReconcileRequirement{}

type DolphinSchedulerClusterReconcileRequirement struct {
	scheme   *runtime.Scheme
	instance *dolphinv1alpha1.DolphinschedulerCluster
	client   client.Client

	roles     []core.RoleReconciler
	resources []core.ResourceReconciler
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

func (d *DolphinSchedulerClusterReconcileRequirement) RegisterRoles() []core.RoleReconciler {
	if len(d.roles) != 0 {
		return d.roles
	}
	roles := make([]core.RoleReconciler, 0)
	roles = append(roles, master.NewMasterRole(d.scheme, d.instance, d.client))   // register master role
	roles = append(roles, worker.NewWorkerRole(d.scheme, d.instance, d.client))   // register master role
	roles = append(roles, api.NewApiRole(d.scheme, d.instance, d.client))         // register api role
	roles = append(roles, alerter.NewAlerterRole(d.scheme, d.instance, d.client)) // register alerter role
	return roles
}

func (d *DolphinSchedulerClusterReconcileRequirement) RegisterResources() []core.ResourceReconciler {
	if len(d.resources) != 0 {
		return d.resources
	}
	lables := d.instance.Labels
	resources := make([]core.ResourceReconciler, 0)
	resources = append(resources, NewConfigMap(d.scheme, d.instance, d.client, lables, nil))
	resources = append(resources, NewServiceAccount(d.scheme, d.instance, d.client, lables, nil))
	resources = append(resources, NewRole(d.scheme, d.instance, d.client, lables, nil))
	resources = append(resources, NewRoleBinding(d.scheme, d.instance, d.client, lables, nil))
	resources = append(resources, NewJobInitScriptReconciler(d.scheme, d.instance, d.client, lables, nil))
	return resources
}
