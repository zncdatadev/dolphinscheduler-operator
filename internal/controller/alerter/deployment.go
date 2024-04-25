package alerter

import (
	"context"
	dolphinv1alpha1 "github.com/zncdata-labs/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdata-labs/dolphinscheduler-operator/internal/common"
	"github.com/zncdata-labs/dolphinscheduler-operator/internal/util"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ common.WorkloadResourceType = &DeploymentReconciler{}

type DeploymentReconciler struct {
	common.WorkloadStyleUncheckedReconciler[*dolphinv1alpha1.DolphinschedulerCluster, *dolphinv1alpha1.RoleGroupSpec]
}

func NewDeployment(
	scheme *runtime.Scheme,
	instance *dolphinv1alpha1.DolphinschedulerCluster,
	client client.Client,
	groupName string,
	labels map[string]string,
	mergedCfg *dolphinv1alpha1.RoleGroupSpec,
	replicate int32,
) *DeploymentReconciler {
	return &DeploymentReconciler{
		WorkloadStyleUncheckedReconciler: *common.NewWorkloadStyleUncheckedReconciler(
			scheme,
			instance,
			client,
			groupName,
			labels,
			mergedCfg,
			replicate,
		),
	}
}

func (s *DeploymentReconciler) Build(_ context.Context) (client.Object, error) {
	builder := common.NewDeploymentBuilder(
		createDeploymentName(s.Instance.GetName(), s.GroupName),
		s.Instance.Namespace,
		s.Labels,
		s.Replicas,
		s.makeAlerterContainer(),
	)
	builder.SetServiceAccountName(common.CreateServiceAccountName(s.Instance.GetName()))
	builder.SetVolumes(s.volumes())
	return builder.Build(), nil
}

func (s *DeploymentReconciler) CommandOverride(resource client.Object) {
	dep := resource.(*appv1.StatefulSet)
	containers := dep.Spec.Template.Spec.Containers
	if cmdOverride := s.MergedCfg.CommandArgsOverrides; cmdOverride != nil {
		for i := range containers {
			if containers[i].Name == string(common.Alerter) {
				containers[i].Command = cmdOverride
				break
			}
		}
	}
}

func (s *DeploymentReconciler) EnvOverride(resource client.Object) {
	dep := resource.(*appv1.StatefulSet)
	containers := dep.Spec.Template.Spec.Containers
	if envOverride := s.MergedCfg.EnvOverrides; envOverride != nil {
		for i := range containers {
			if containers[i].Name == string(common.Alerter) {
				envVars := containers[i].Env
				common.OverrideEnvVars(&envVars, s.MergedCfg.EnvOverrides)
				break
			}
		}
	}
}

func (s *DeploymentReconciler) LogOverride(_ client.Object) {
	// do nothing, see name node
}

func (s *DeploymentReconciler) makeAlerterContainer() []corev1.Container {
	imageSpec := s.Instance.Spec.Alerter.Image
	resourceSpec := s.MergedCfg.Config.Resources
	zNode := s.Instance.Spec.ClusterConfigSpec.ZookeeperDiscoveryZNode
	imageName := util.ImageRepository(imageSpec.Repository, imageSpec.Tag)
	configConfigMapName := common.ConfigConfigMapName(s.Instance.GetName(), s.GroupName)
	envsConfigMapName := common.EnvsConfigMapName(s.Instance.GetName(), s.GroupName)
	builder := NewAlerterContainerBuilder(
		imageName,
		imageSpec.PullPolicy,
		zNode,
		resourceSpec,
		envsConfigMapName,
		configConfigMapName,
		s.Instance.Spec.ClusterConfigSpec.Database,
	)
	dolphinContainer := builder.Build(builder)
	return []corev1.Container{
		dolphinContainer,
	}
}

// make volumes
func (s *DeploymentReconciler) volumes() []common.VolumeSpec {
	return []common.VolumeSpec{
		{
			Name:       configVolumeName(),
			SourceType: common.ConfigMap,
			Params: &common.VolumeSourceParams{
				ConfigMap: common.ConfigMapSpec{
					Name: common.ConfigConfigMapName(s.Instance.GetName(), s.GroupName),
				}},
		},
	}
}

func (s *DeploymentReconciler) GetConditions() *[]metav1.Condition {
	return &s.Instance.Status.Conditions
}
