package master

import (
	"context"
	dolphinv1alpha1 "github.com/zncdata-labs/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdata-labs/dolphinscheduler-operator/internal/common"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/core"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/resource"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewMasterConfigMap(
	scheme *runtime.Scheme,
	instance *dolphinv1alpha1.DolphinschedulerCluster,
	client client.Client,
	groupName string,
	labels map[string]string,
	mergedCfg *dolphinv1alpha1.RoleGroupSpec,
) *ConfigMapReconciler {
	return &ConfigMapReconciler{
		MultiResourceReconciler: *core.NewMultiResourceReconciler(
			scheme,
			instance,
			client,
			groupName,
			labels,
			mergedCfg,
		),
	}
}

var _ core.MultiResourceReconcilerBuilder = &ConfigMapReconciler{}

type ConfigMapReconciler struct {
	core.MultiResourceReconciler[*dolphinv1alpha1.DolphinschedulerCluster, *dolphinv1alpha1.RoleGroupSpec]
}

func (c *ConfigMapReconciler) Build(ctx context.Context) ([]core.ResourceBuilder, error) {
	return []core.ResourceBuilder{
		c.createEnvConfigMapReconciler(),
		c.createConfigConfigMapReconciler(),
	}, nil
}

// create env configmap
func (c *ConfigMapReconciler) createEnvConfigMapReconciler() core.ResourceBuilder {
	cm := resource.NewGeneralConfigMap(
		c.Scheme,
		c.Instance,
		c.Client,
		c.GroupName,
		c.Labels,
		c.MergedCfg,
		c.createEnvConfigMap,
		nil, // todo
	)
	return cm
}

func (c *ConfigMapReconciler) createEnvConfigMap() (*corev1.ConfigMap, error) {
	var generators []interface{}
	generators = append(generators, common.EnvPropertiesGenerator{})
	builder := resource.NewConfigMapBuilder(
		common.EnvsConfigMapName(c.Instance.GetName(), c.GroupName),
		c.Instance.Namespace,
		c.Labels,
		generators,
	)
	return builder.Build(), nil
}

// crate config configmap
func (c *ConfigMapReconciler) createConfigConfigMapReconciler() core.ResourceBuilder {
	cm := resource.NewGeneralConfigMap(
		c.Scheme,
		c.Instance,
		c.Client,
		c.GroupName,
		c.Labels,
		c.MergedCfg,
		c.createConfigConfigMap,
		nil, // todo
	)
	return cm
}

func (c *ConfigMapReconciler) createConfigConfigMap() (*corev1.ConfigMap, error) {
	var generators []interface{}
	generators = append(generators, common.ConfigPropertiesGenerator{})
	builder := resource.NewConfigMapBuilder(
		common.ConfigConfigMapName(c.Instance.GetName(), c.GroupName),
		c.Instance.Namespace,
		c.Labels,
		generators,
	)
	return builder.Build(), nil
}
