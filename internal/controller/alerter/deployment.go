package alerter

import (
	"context"
	dolphinv1alpha1 "github.com/zncdata-labs/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdata-labs/dolphinscheduler-operator/internal/common"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/core"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/resource"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/util"
	appv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ resource.WorkloadResourceType = &DeploymentReconciler{}

type DeploymentReconciler struct {
	core.WorkloadStyleUncheckedReconciler[*dolphinv1alpha1.DolphinschedulerCluster, *dolphinv1alpha1.RoleGroupSpec]
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
		WorkloadStyleUncheckedReconciler: *core.NewWorkloadStyleUncheckedReconciler(
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
	builder := resource.NewDeploymentBuilder(
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

func (s *DeploymentReconciler) CommandOverride(obj client.Object) {
	dep := obj.(*appv1.StatefulSet)
	containers := dep.Spec.Template.Spec.Containers
	if cmdOverride := s.MergedCfg.CommandArgsOverrides; cmdOverride != nil {
		for i := range containers {
			if containers[i].Name == string(core.Alerter) {
				containers[i].Command = cmdOverride
				break
			}
		}
	}
}

func (s *DeploymentReconciler) EnvOverride(obj client.Object) {
	dep := obj.(*appv1.StatefulSet)
	containers := dep.Spec.Template.Spec.Containers
	if envOverride := s.MergedCfg.EnvOverrides; envOverride != nil {
		for i := range containers {
			if containers[i].Name == string(core.Alerter) {
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
func (s *DeploymentReconciler) volumes() []resource.VolumeSpec {
	return []resource.VolumeSpec{
		{
			Name:       configVolumeName(),
			SourceType: resource.ConfigMap,
			Params: &resource.VolumeSourceParams{
				ConfigMap: resource.ConfigMapSpec{
					Name: common.ConfigConfigMapName(s.Instance.GetName(), s.GroupName),
				}},
		},
	}
}

func (s *DeploymentReconciler) GetConditions() *[]metav1.Condition {
	return &s.Instance.Status.Conditions
}
