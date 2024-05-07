package master

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

func NewMasterRole(
	scheme *runtime.Scheme,
	instance *dolphinv1alpha1.DolphinschedulerCluster,
	client client.Client) *core.BaseRoleReconciler[*dolphinv1alpha1.DolphinschedulerCluster] {
	dolphinInstance := &common.DolphinSchedulerClusterInstance{Instance: instance}
	LabelHelper := core.RoleLabelHelper{}
	roleLabels := LabelHelper.RoleLabels(instance.GetName(), getRole())
	masterHelper := NewRoleMasterRequirements(scheme, instance, roleLabels, client)

	var pdb core.ResourceReconciler
	if instance.Spec.Alerter.PodDisruptionBudget != nil {
		pdb = resource.NewReconcilePDB(client, scheme, instance, roleLabels, string(getRole()), common.PdbCfg(instance.Spec.Alerter.PodDisruptionBudget))
	}
	return core.NewBaseRoleReconciler(scheme, instance, client, getRole(), roleLabels, dolphinInstance, masterHelper, pdb)
}

func NewRoleMasterRequirements(scheme *runtime.Scheme, instance *dolphinv1alpha1.DolphinschedulerCluster,
	roleLabels map[string]string, client client.Client) *RoleMasterRequirements {
	return &RoleMasterRequirements{
		scheme:          scheme,
		instance:        instance,
		client:          client,
		groups:          maps.Keys(instance.Spec.Master.RoleGroups),
		roleLabels:      roleLabels,
		masterRoleSpec:  instance.Spec.Master,
		masterGroupSpec: instance.Spec.Master.RoleGroups,
	}
}

var _ core.RoleReconcilerRequirements = &RoleMasterRequirements{}

type RoleMasterRequirements struct {
	scheme          *runtime.Scheme
	instance        *dolphinv1alpha1.DolphinschedulerCluster
	client          client.Client
	groups          []string
	roleLabels      map[string]string
	masterRoleSpec  *dolphinv1alpha1.MasterSpec
	masterGroupSpec map[string]*dolphinv1alpha1.MasterRoleGroupSpec
}

func (r *RoleMasterRequirements) MergeConfig() map[string]any {
	var mergedCfg = make(map[string]any)
	for groupName, cfg := range r.masterGroupSpec {
		copiedRoleGroup := cfg.DeepCopy()
		// Merge the role into the role group.
		// if the role group has a config, and role group not has a config, will
		// merge the role's config into the role group's config.
		core.MergeObjects(copiedRoleGroup, r.masterRoleSpec, []string{"RoleGroups"})

		// merge the role's config into the role group's config
		if r.masterRoleSpec.Config != nil && copiedRoleGroup.Config != nil {
			core.MergeObjects(copiedRoleGroup.Config, r.masterRoleSpec.Config, []string{})
		}
		mergedCfg[groupName] = copiedRoleGroup
	}
	return mergedCfg
}

func (r *RoleMasterRequirements) RegisterResources(ctx context.Context) map[string][]core.ResourceReconciler {
	var reconcilers = map[string][]core.ResourceReconciler{}
	helper := core.RoleLabelHelper{}
	for _, groupName := range r.groups {
		value := core.GetRoleGroup(r.instance.Name, getRole(), groupName)
		mergedCfg := value.(*dolphinv1alpha1.MasterRoleGroupSpec)
		labels := helper.GroupLabels(r.roleLabels, groupName, mergedCfg.Config.NodeSelector)
		cm := NewMasterConfigMap(r.scheme, r.instance, r.client, groupName, labels, mergedCfg)
		statefulset := NewStatefulSet(ctx, r.scheme, r.instance, r.client, groupName, labels, mergedCfg, mergedCfg.Replicas)
		svc := NewMasterServiceHeadless(r.scheme, r.instance, r.client, groupName, labels, mergedCfg)
		logging := NewMasterLogging(r.scheme, r.instance, r.client, groupName, labels, mergedCfg)
		groupReconcilers := []core.ResourceReconciler{cm, logging, statefulset, svc}
		if mergedCfg.Config.PodDisruptionBudget != nil {
			pdb := resource.NewReconcilePDB(r.client, r.scheme, r.instance, labels, groupName,
				common.PdbCfg(mergedCfg.Config.PodDisruptionBudget))
			groupReconcilers = append(groupReconcilers, pdb)
		}
		reconcilers[groupName] = groupReconcilers
	}
	return reconcilers
}
