package master

import (
	"context"
	dolphinv1alpha1 "github.com/zncdata-labs/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdata-labs/dolphinscheduler-operator/internal/common"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/core"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/resource"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewStatefulSet(
	ctx context.Context,
	scheme *runtime.Scheme,
	instance *dolphinv1alpha1.DolphinschedulerCluster,
	client client.Client,
	groupName string,
	labels map[string]string,
	mergedCfg *dolphinv1alpha1.MasterRoleGroupSpec,
	replicate int32,
) *resource.GenericStatefulSetReconciler[*dolphinv1alpha1.DolphinschedulerCluster, *dolphinv1alpha1.MasterRoleGroupSpec] {
	requirements := newStatefulSetBuilderRequirements(ctx, instance, client, mergedCfg, groupName, labels, replicate)
	return resource.NewGenericStatefulSetReconciler(scheme, instance, client, groupName, labels, mergedCfg, replicate,
		requirements)
}

func newStatefulSetBuilderRequirements(
	ctx context.Context, instance *dolphinv1alpha1.DolphinschedulerCluster, client client.Client,
	mergedCfg *dolphinv1alpha1.MasterRoleGroupSpec, groupName string,
	labels map[string]string, replicas int32) *StatefulSetBuilderRequirements {
	containers := createContainers(instance, groupName, client, mergedCfg, ctx)
	workloadResourceRequirements := resource.NewGenericWorkloadRequirements(string(core.Master), &containers,
		mergedCfg.CommandArgsOverrides, mergedCfg.EnvOverrides, &instance.Status.Conditions)
	return &StatefulSetBuilderRequirements{
		instance:                     instance,
		groupName:                    groupName,
		labels:                       labels,
		replicas:                     replicas,
		containers:                   containers,
		WorkloadResourceRequirements: workloadResourceRequirements,
	}
}

var _ resource.WorkloadBuilderRequirements = &StatefulSetBuilderRequirements{}

type StatefulSetBuilderRequirements struct {
	instance   *dolphinv1alpha1.DolphinschedulerCluster
	labels     map[string]string
	groupName  string
	replicas   int32
	containers []corev1.Container
	core.WorkloadResourceRequirements
}

func (s *StatefulSetBuilderRequirements) Build(ctx context.Context) (client.Object, error) {
	builder := s.createStatefulSetBuilder(s.containers)
	return builder.Build(ctx)
}

func (s *StatefulSetBuilderRequirements) createStatefulSetBuilder(containers []corev1.Container) *resource.StatefulSetBuilder {
	builder := resource.NewStatefulSetBuilder(
		createStatefulSetName(s.instance.GetName(), s.groupName),
		s.instance.Namespace,
		s.labels,
		s.replicas,
		createSvcName(s.instance.GetName(), s.groupName),
		containers,
	)
	builder.SetServiceAccountName(common.CreateServiceAccountName(s.instance.GetName()))
	builder.SetVolumes(volumes(s.instance.GetName(), s.groupName))
	return builder
}

func createContainers(
	instance *dolphinv1alpha1.DolphinschedulerCluster,
	groupName string,
	client client.Client,
	mergedCfg *dolphinv1alpha1.MasterRoleGroupSpec,
	ctx context.Context) []corev1.Container {
	imageSpec := instance.Spec.Master.Image
	resourceSpec := mergedCfg.Config.Resources
	zNode := instance.Spec.ClusterConfigSpec.ZookeeperDiscoveryZNode
	imageName := util.ImageRepository(imageSpec.Repository, imageSpec.Tag)
	configConfigMapName := common.ConfigConfigMapName(instance.GetName(), groupName)
	envsConfigMapName := common.EnvsConfigMapName(instance.GetName(), groupName)
	_, dbParams := common.ExtractDataBaseReference(instance.Spec.ClusterConfigSpec.Database, ctx, client, instance.GetNamespace())
	containerBuilder := NewMasterContainerBuilder(
		imageName,
		imageSpec.PullPolicy,
		zNode,
		resourceSpec,
		envsConfigMapName,
		configConfigMapName,
		dbParams,
	)
	dolphinContainer := containerBuilder.Build(containerBuilder)
	return []corev1.Container{dolphinContainer}
}

// make volumes
func volumes(instanceName string, groupName string) []resource.VolumeSpec {
	return []resource.VolumeSpec{
		{
			Name:       configVolumeName(),
			SourceType: resource.ConfigMap,
			Params: &resource.VolumeSourceParams{
				ConfigMap: resource.ConfigMapSpec{
					Name: common.ConfigConfigMapName(instanceName, groupName),
				}},
		},
	}
}
