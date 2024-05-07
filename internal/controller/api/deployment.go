package api

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

func NewDeployment(
	ctx context.Context,
	scheme *runtime.Scheme,
	instance *dolphinv1alpha1.DolphinschedulerCluster,
	client client.Client,
	groupName string,
	labels map[string]string,
	mergedCfg *dolphinv1alpha1.ApiRoleGroupSpec,
	replicate int32,
) *resource.GenericDeploymentReconciler[*dolphinv1alpha1.DolphinschedulerCluster, *dolphinv1alpha1.ApiRoleGroupSpec] {
	requirements := newDeploymentBuilderRequirements(ctx, instance, client, mergedCfg, groupName, labels, replicate)
	return resource.NewGenericDeploymentReconciler(scheme, instance, client, groupName, labels, mergedCfg, replicate, requirements)
}

func newDeploymentBuilderRequirements(
	ctx context.Context, instance *dolphinv1alpha1.DolphinschedulerCluster, client client.Client,
	mergedCfg *dolphinv1alpha1.ApiRoleGroupSpec, groupName string,
	labels map[string]string, replicas int32) *DeploymentBuilderRequirements {
	containers := createContainers(instance, groupName, client, mergedCfg, ctx)
	workloadResourceRequirements := resource.NewGenericWorkloadRequirements(string(core.Api), &containers,
		mergedCfg.CommandArgsOverrides, mergedCfg.EnvOverrides, &instance.Status.Conditions)
	// optional, set logging override handler
	workloadResourceRequirements.LoggingOverrideHandler = resource.NewLoggingOverrideHandler(logbackConfigVolumeName(),
		logbackConfigMapName(instance.GetName(), groupName), logbackMountPath(),
		dolphinv1alpha1.LogbackPropertiesFileName)
	return &DeploymentBuilderRequirements{
		instance:                     instance,
		groupName:                    groupName,
		labels:                       labels,
		replicas:                     replicas,
		containers:                   containers,
		WorkloadResourceRequirements: workloadResourceRequirements,
	}
}

var _ resource.WorkloadBuilderRequirements = &DeploymentBuilderRequirements{}

type DeploymentBuilderRequirements struct {
	instance   *dolphinv1alpha1.DolphinschedulerCluster
	labels     map[string]string
	groupName  string
	replicas   int32
	containers []corev1.Container
	core.WorkloadResourceRequirements
}

func (s *DeploymentBuilderRequirements) Build(ctx context.Context) (client.Object, error) {
	builder := s.createDeploymentBuilder(s.containers)
	return builder.Build(ctx)
}

func (s *DeploymentBuilderRequirements) createDeploymentBuilder(containers []corev1.Container) *resource.DeploymentBuilder {
	builder := resource.NewDeploymentBuilder(
		createDeploymentName(s.instance.GetName(), s.groupName),
		s.instance.Namespace,
		s.labels,
		s.replicas,
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
	mergedCfg *dolphinv1alpha1.ApiRoleGroupSpec,
	ctx context.Context) []corev1.Container {
	imageSpec := instance.Spec.Api.Image
	resourceSpec := mergedCfg.Config.Resources
	zNode := instance.Spec.ClusterConfigSpec.ZookeeperDiscoveryZNode
	imageName := util.ImageRepository(imageSpec.Repository, imageSpec.Tag)
	configConfigMapName := common.ConfigConfigMapName(instance.GetName(), groupName)
	envsConfigMapName := common.EnvsConfigMapName(instance.GetName(), groupName)
	_, dbParams := common.ExtractDataBaseReference(instance.Spec.ClusterConfigSpec.Database, ctx, client, instance.GetNamespace())
	containerBuilder := NewApiContainerBuilder(
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
