package alerter

import (
	"context"
	dolphinv1alpha1 "github.com/zncdata-labs/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdata-labs/dolphinscheduler-operator/internal/common"
	"golang.org/x/exp/maps"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewAlerterRole(
	scheme *runtime.Scheme,
	instance *dolphinv1alpha1.DolphinschedulerCluster,
	client client.Client) *common.BaseRoleReconciler[*dolphinv1alpha1.DolphinschedulerCluster] {
	dolphinInstance := &common.DolphinSchedulerClusterInstance{Instance: instance}
	LabelHelper := common.RoleLabelHelper{}
	roleLabels := LabelHelper.RoleLabels(instance.GetName(), common.Alerter)
	alerterHelper := NewRoleAlerterHelper(scheme, instance, roleLabels, client)
	return common.NewBaseRoleReconciler(scheme, instance, client, common.Alerter, roleLabels, dolphinInstance, alerterHelper)
}

func NewRoleAlerterHelper(scheme *runtime.Scheme, instance *dolphinv1alpha1.DolphinschedulerCluster,
	roleLabels map[string]string, client client.Client) *RoleAlerterHelper {
	return &RoleAlerterHelper{
		scheme:           scheme,
		instance:         instance,
		client:           client,
		groups:           maps.Keys(instance.Spec.Alerter.RoleGroups),
		roleLabels:       roleLabels,
		alerterRoleSpec:  instance.Spec.Alerter,
		alerterGroupSpec: instance.Spec.Alerter.RoleGroups,
	}
}

var _ common.RoleHelper = &RoleAlerterHelper{}

type RoleAlerterHelper struct {
	scheme           *runtime.Scheme
	instance         *dolphinv1alpha1.DolphinschedulerCluster
	client           client.Client
	groups           []string
	roleLabels       map[string]string
	alerterRoleSpec  *dolphinv1alpha1.AlerterSpec
	alerterGroupSpec map[string]*dolphinv1alpha1.RoleGroupSpec
}

func (r *RoleAlerterHelper) MergeConfig() map[string]any {
	var mergedCfg = make(map[string]any)
	for groupName, cfg := range r.alerterGroupSpec {
		copiedRoleGroup := cfg.DeepCopy()
		// Merge the role into the role group.
		// if the role group has a config, and role group not has a config, will
		// merge the role's config into the role group's config.
		common.MergeObjects(copiedRoleGroup, r.alerterRoleSpec, []string{"RoleGroups"})

		// merge the role's config into the role group's config
		if r.alerterRoleSpec.Config != nil && copiedRoleGroup.Config != nil {
			common.MergeObjects(copiedRoleGroup.Config, r.alerterRoleSpec.Config, []string{})
		}
		mergedCfg[groupName] = copiedRoleGroup
	}
	return mergedCfg
}

func (r *RoleAlerterHelper) RegisterResources(ctx context.Context) map[string][]common.ResourceReconciler {
	var reconcilers = map[string][]common.ResourceReconciler{}
	helper := common.RoleLabelHelper{}
	for _, groupName := range r.groups {
		value := common.GetRoleGroup(r.instance.Name, common.Alerter, groupName)
		mergedCfg := value.(*dolphinv1alpha1.RoleGroupSpec)
		labels := helper.GroupLabels(r.roleLabels, groupName, mergedCfg.Config.NodeSelector)
		statefulset := NewDeployment(r.scheme, r.instance, r.client, groupName, labels, mergedCfg, mergedCfg.Replicas)
		svc := NewAlerterService(r.scheme, r.instance, r.client, groupName, labels, mergedCfg)
		groupReconcilers := []common.ResourceReconciler{statefulset, svc}
		if mergedCfg.Config.PodDisruptionBudget != nil {
			pdb := common.NewReconcilePDB(r.client, r.scheme, r.instance, labels, groupName,
				common.PdbCfg(mergedCfg.Config.PodDisruptionBudget))
			groupReconcilers = append(groupReconcilers, pdb)
		}
		reconcilers[groupName] = groupReconcilers
	}
	return reconcilers
}
