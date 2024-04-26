package api

import (
	"context"
	dolphinv1alpha1 "github.com/zncdata-labs/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdata-labs/dolphinscheduler-operator/internal/common"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/core"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/resource"
	"golang.org/x/exp/maps"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewApiRole(
	scheme *runtime.Scheme,
	instance *dolphinv1alpha1.DolphinschedulerCluster,
	client client.Client) *core.BaseRoleReconciler[*dolphinv1alpha1.DolphinschedulerCluster] {
	dolphinInstance := &common.DolphinSchedulerClusterInstance{Instance: instance}
	LabelHelper := core.RoleLabelHelper{}
	roleLabels := LabelHelper.RoleLabels(instance.GetName(), core.Api)
	apiHelper := NewRoleApiHelper(scheme, instance, roleLabels, client)
	pdb := resource.NewReconcilePDB(client, scheme, instance, roleLabels, string(core.Api), common.PdbCfg(instance.Spec.Alerter.PodDisruptionBudget))
	return core.NewBaseRoleReconciler(scheme, instance, client, core.Api, roleLabels, dolphinInstance, apiHelper, pdb)
}

func NewRoleApiHelper(scheme *runtime.Scheme, instance *dolphinv1alpha1.DolphinschedulerCluster,
	roleLabels map[string]string, client client.Client) *RoleApiHelper {
	return &RoleApiHelper{
		scheme:       scheme,
		instance:     instance,
		client:       client,
		groups:       maps.Keys(instance.Spec.Api.RoleGroups),
		roleLabels:   roleLabels,
		apiRoleSpec:  instance.Spec.Api,
		apiGroupSpec: instance.Spec.Api.RoleGroups,
	}
}

var _ core.RoleHelper = &RoleApiHelper{}

type RoleApiHelper struct {
	scheme       *runtime.Scheme
	instance     *dolphinv1alpha1.DolphinschedulerCluster
	client       client.Client
	groups       []string
	roleLabels   map[string]string
	apiRoleSpec  *dolphinv1alpha1.ApiSpec
	apiGroupSpec map[string]*dolphinv1alpha1.RoleGroupSpec
}

func (r *RoleApiHelper) MergeConfig() map[string]any {
	var mergedCfg = make(map[string]any)
	for groupName, cfg := range r.apiGroupSpec {
		copiedRoleGroup := cfg.DeepCopy()
		// Merge the role into the role group.
		// if the role group has a config, and role group not has a config, will
		// merge the role's config into the role group's config.
		core.MergeObjects(copiedRoleGroup, r.apiRoleSpec, []string{"RoleGroups"})

		// merge the role's config into the role group's config
		if r.apiRoleSpec.Config != nil && copiedRoleGroup.Config != nil {
			core.MergeObjects(copiedRoleGroup.Config, r.apiRoleSpec.Config, []string{})
		}
		mergedCfg[groupName] = copiedRoleGroup
	}
	return mergedCfg
}

func (r *RoleApiHelper) RegisterResources(ctx context.Context) map[string][]core.ResourceReconciler {
	var reconcilers = map[string][]core.ResourceReconciler{}
	helper := core.RoleLabelHelper{}
	for _, groupName := range r.groups {
		value := core.GetRoleGroup(r.instance.Name, core.Api, groupName)
		mergedCfg := value.(*dolphinv1alpha1.RoleGroupSpec)
		labels := helper.GroupLabels(r.roleLabels, groupName, mergedCfg.Config.NodeSelector)
		statefulset := NewDeployment(r.scheme, r.instance, r.client, groupName, labels, mergedCfg, mergedCfg.Replicas)
		svc := NewApiService(r.scheme, r.instance, r.client, groupName, labels, mergedCfg)
		ingress := NewIngress(r.scheme, r.instance, r.client, groupName, labels, mergedCfg)
		groupReconcilers := []core.ResourceReconciler{statefulset, svc, ingress}
		if mergedCfg.Config.PodDisruptionBudget != nil {
			pdb := resource.NewReconcilePDB(r.client, r.scheme, r.instance, labels, groupName,
				common.PdbCfg(mergedCfg.Config.PodDisruptionBudget))
			groupReconcilers = append(groupReconcilers, pdb)
		}
		reconcilers[groupName] = groupReconcilers
	}
	return reconcilers
}
