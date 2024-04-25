package worker

import (
	"context"
	dolphinv1alpha1 "github.com/zncdata-labs/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdata-labs/dolphinscheduler-operator/internal/common"
	"golang.org/x/exp/maps"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewWorkerRole(
	scheme *runtime.Scheme,
	instance *dolphinv1alpha1.DolphinschedulerCluster,
	client client.Client) *common.BaseRoleReconciler[*dolphinv1alpha1.DolphinschedulerCluster] {
	dolphinInstance := &common.DolphinSchedulerClusterInstance{Instance: instance}
	LabelHelper := common.RoleLabelHelper{}
	roleLabels := LabelHelper.RoleLabels(instance.GetName(), common.Worker)
	workerHelper := NewRoleWorkerHelper(scheme, instance, roleLabels, client)
	return common.NewBaseRoleReconciler(scheme, instance, client, common.Worker, roleLabels, dolphinInstance, workerHelper)
}

func NewRoleWorkerHelper(scheme *runtime.Scheme, instance *dolphinv1alpha1.DolphinschedulerCluster,
	roleLabels map[string]string, client client.Client) *RoleWorkerHelper {
	return &RoleWorkerHelper{
		scheme:          scheme,
		instance:        instance,
		client:          client,
		groups:          maps.Keys(instance.Spec.Worker.RoleGroups),
		roleLabels:      roleLabels,
		workerRoleSpec:  instance.Spec.Worker,
		workerGroupSpec: instance.Spec.Worker.RoleGroups,
	}
}

var _ common.RoleHelper = &RoleWorkerHelper{}

type RoleWorkerHelper struct {
	scheme          *runtime.Scheme
	instance        *dolphinv1alpha1.DolphinschedulerCluster
	client          client.Client
	groups          []string
	roleLabels      map[string]string
	workerRoleSpec  *dolphinv1alpha1.WorkerSpec
	workerGroupSpec map[string]*dolphinv1alpha1.RoleGroupSpec
}

func (r *RoleWorkerHelper) MergeConfig() map[string]any {
	var mergedCfg = make(map[string]any)
	for groupName, cfg := range r.workerGroupSpec {
		copiedRoleGroup := cfg.DeepCopy()
		// Merge the role into the role group.
		// if the role group has a config, and role group not has a config, will
		// merge the role's config into the role group's config.
		common.MergeObjects(copiedRoleGroup, r.workerRoleSpec, []string{"RoleGroups"})

		// merge the role's config into the role group's config
		if r.workerRoleSpec.Config != nil && copiedRoleGroup.Config != nil {
			common.MergeObjects(copiedRoleGroup.Config, r.workerRoleSpec.Config, []string{})
		}
		mergedCfg[groupName] = copiedRoleGroup
	}
	return mergedCfg
}

func (r *RoleWorkerHelper) RegisterResources(ctx context.Context) map[string][]common.ResourceReconciler {
	var reconcilers = map[string][]common.ResourceReconciler{}
	helper := common.RoleLabelHelper{}
	for _, groupName := range r.groups {
		value := common.GetRoleGroup(r.instance.Name, common.Worker, groupName)
		mergedCfg := value.(*dolphinv1alpha1.RoleGroupSpec)
		labels := helper.GroupLabels(r.roleLabels, groupName, mergedCfg.Config.NodeSelector)
		statefulset := NewStatefulSet(r.scheme, r.instance, r.client, groupName, labels, mergedCfg, mergedCfg.Replicas)
		svc := NewWorkerServiceHeadless(r.scheme, r.instance, r.client, groupName, labels, mergedCfg)
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
