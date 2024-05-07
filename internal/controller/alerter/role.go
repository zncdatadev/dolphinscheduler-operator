package alerter

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

func NewAlerterRole(
	scheme *runtime.Scheme,
	instance *dolphinv1alpha1.DolphinschedulerCluster,
	client client.Client) *core.BaseRoleReconciler[*dolphinv1alpha1.DolphinschedulerCluster] {
	dolphinInstance := &common.DolphinSchedulerClusterInstance{Instance: instance}
	LabelHelper := core.RoleLabelHelper{}
	roleLabels := LabelHelper.RoleLabels(instance.GetName(), core.Alerter)
	alerterHelper := NewRoleAlerterRequirements(scheme, instance, roleLabels, client)

	var pdb core.ResourceReconciler
	if instance.Spec.Alerter.PodDisruptionBudget != nil {
		pdb = resource.NewReconcilePDB(client, scheme, instance, roleLabels, string(core.Alerter), common.PdbCfg(instance.Spec.Alerter.PodDisruptionBudget))
	}
	return core.NewBaseRoleReconciler(scheme, instance, client, core.Alerter, roleLabels, dolphinInstance, alerterHelper, pdb)
}

func NewRoleAlerterRequirements(scheme *runtime.Scheme, instance *dolphinv1alpha1.DolphinschedulerCluster,
	roleLabels map[string]string, client client.Client) *RoleAlerterRequirements {
	return &RoleAlerterRequirements{
		scheme:           scheme,
		instance:         instance,
		client:           client,
		groups:           maps.Keys(instance.Spec.Alerter.RoleGroups),
		roleLabels:       roleLabels,
		alerterRoleSpec:  instance.Spec.Alerter,
		alerterGroupSpec: instance.Spec.Alerter.RoleGroups,
	}
}

var _ core.RoleReconcilerRequirements = &RoleAlerterRequirements{}

type RoleAlerterRequirements struct {
	scheme           *runtime.Scheme
	instance         *dolphinv1alpha1.DolphinschedulerCluster
	client           client.Client
	groups           []string
	roleLabels       map[string]string
	alerterRoleSpec  *dolphinv1alpha1.AlerterSpec
	alerterGroupSpec map[string]*dolphinv1alpha1.AlerterRoleGroupSpec
}

func (r *RoleAlerterRequirements) MergeConfig() map[string]any {
	var mergedCfg = make(map[string]any)
	for groupName, cfg := range r.alerterGroupSpec {
		copiedRoleGroup := cfg.DeepCopy()
		// Merge the role into the role group.
		// if the role group has a config, and role group not has a config, will
		// merge the role's config into the role group's config.
		core.MergeObjects(copiedRoleGroup, r.alerterRoleSpec, []string{"RoleGroups"})

		// merge the role's config into the role group's config
		if r.alerterRoleSpec.Config != nil && copiedRoleGroup.Config != nil {
			core.MergeObjects(copiedRoleGroup.Config, r.alerterRoleSpec.Config, []string{})
		}
		mergedCfg[groupName] = copiedRoleGroup
	}
	return mergedCfg
}

func (r *RoleAlerterRequirements) RegisterResources(ctx context.Context) map[string][]core.ResourceReconciler {
	var reconcilers = map[string][]core.ResourceReconciler{}
	helper := core.RoleLabelHelper{}
	for _, groupName := range r.groups {
		value := core.GetRoleGroup(r.instance.Name, core.Alerter, groupName)
		mergedCfg := value.(*dolphinv1alpha1.AlerterRoleGroupSpec)
		labels := helper.GroupLabels(r.roleLabels, groupName, mergedCfg.Config.NodeSelector)
		logging := NewAlerterLogging(r.scheme, r.instance, r.client, groupName, labels, mergedCfg)
		statefulset := NewDeployment(ctx, r.scheme, r.instance, r.client, groupName, labels, mergedCfg, mergedCfg.Replicas)
		svc := NewAlerterService(r.scheme, r.instance, r.client, groupName, labels, mergedCfg)
		groupReconcilers := []core.ResourceReconciler{logging, statefulset, svc}
		if mergedCfg.Config.PodDisruptionBudget != nil {
			pdb := resource.NewReconcilePDB(r.client, r.scheme, r.instance, labels, groupName,
				common.PdbCfg(mergedCfg.Config.PodDisruptionBudget))
			groupReconcilers = append(groupReconcilers, pdb)
		}
		reconcilers[groupName] = groupReconcilers
	}
	return reconcilers
}
