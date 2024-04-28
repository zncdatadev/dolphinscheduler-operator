package worker

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

var _ resource.WorkloadResourceType = &StatefulSetReconciler{}

type StatefulSetReconciler struct {
	core.WorkloadStyleReconciler[*dolphinv1alpha1.DolphinschedulerCluster, *dolphinv1alpha1.WorkerRoleGroupSpec]
}

func NewStatefulSet(
	scheme *runtime.Scheme,
	instance *dolphinv1alpha1.DolphinschedulerCluster,
	client client.Client,
	groupName string,
	labels map[string]string,
	mergedCfg *dolphinv1alpha1.WorkerRoleGroupSpec,
	replicate int32,
) *StatefulSetReconciler {
	return &StatefulSetReconciler{
		WorkloadStyleReconciler: *core.NewWorkloadStyleReconciler(
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

func (s *StatefulSetReconciler) Build(ctx context.Context) (client.Object, error) {
	builder := resource.NewStatefulSetBuilder(
		createStatefulSetName(s.Instance.GetName(), s.GroupName),
		s.Instance.Namespace,
		s.Labels,
		s.Replicas,
		createSvcName(s.Instance.GetName(), s.GroupName),
		s.makeWorkerContainer(ctx),
	)
	builder.SetServiceAccountName(common.CreateServiceAccountName(s.Instance.GetName()))
	builder.SetVolumes(s.volumes())
	builder.SetPvcTemplates(s.pvcTemplates())
	return builder.Build(), nil
}

func (s *StatefulSetReconciler) CommandOverride(obj client.Object) {
	dep := obj.(*appv1.StatefulSet)
	containers := dep.Spec.Template.Spec.Containers
	if cmdOverride := s.MergedCfg.CommandArgsOverrides; cmdOverride != nil {
		for i := range containers {
			if containers[i].Name == string(core.Worker) {
				containers[i].Command = cmdOverride
				break
			}
		}
	}
}

func (s *StatefulSetReconciler) EnvOverride(obj client.Object) {
	dep := obj.(*appv1.StatefulSet)
	containers := dep.Spec.Template.Spec.Containers
	if envOverride := s.MergedCfg.EnvOverrides; envOverride != nil {
		for i := range containers {
			if containers[i].Name == string(core.Worker) {
				envVars := containers[i].Env
				common.OverrideEnvVars(&envVars, s.MergedCfg.EnvOverrides)
				break
			}
		}
	}
}

func (s *StatefulSetReconciler) LogOverride(_ client.Object) {
	// do nothing, see name node
}

func (s *StatefulSetReconciler) makeWorkerContainer(ctx context.Context) []corev1.Container {
	imageSpec := s.Instance.Spec.Worker.Image
	resourceSpec := s.MergedCfg.Config.Resources
	zNode := s.Instance.Spec.ClusterConfigSpec.ZookeeperDiscoveryZNode
	imageName := util.ImageRepository(imageSpec.Repository, imageSpec.Tag)
	configConfigMapName := common.ConfigConfigMapName(s.Instance.GetName(), s.GroupName)
	envsConfigMapName := common.EnvsConfigMapName(s.Instance.GetName(), s.GroupName)
	_, dbParams := common.ExtractDataBaseReference(s.Instance.Spec.ClusterConfigSpec.Database, ctx, s.Client, s.Instance.GetNamespace())
	builder := NewWorkerContainerBuilder(
		imageName,
		imageSpec.PullPolicy,
		zNode,
		resourceSpec,
		envsConfigMapName,
		configConfigMapName,
		dbParams,
	)
	dolphinContainer := builder.Build(builder)
	return []corev1.Container{
		dolphinContainer,
	}
}

// make volumes
func (s *StatefulSetReconciler) volumes() []resource.VolumeSpec {
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

// make pvc templates
func (s *StatefulSetReconciler) pvcTemplates() []resource.VolumeClaimTemplateSpec {
	return []resource.VolumeClaimTemplateSpec{
		{
			Name: workerDataVolumeName(),
			PvcSpec: resource.PvcSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				StorageSize: s.MergedCfg.Config.StorageSize,
			},
		},
	}
}

func (s *StatefulSetReconciler) GetConditions() *[]metav1.Condition {
	return &s.Instance.Status.Conditions
}
