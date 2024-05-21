package master

import (
	"context"

	dolphinv1alpha1 "github.com/zncdatadev/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdatadev/dolphinscheduler-operator/internal/common"
	"github.com/zncdatadev/dolphinscheduler-operator/pkg/core"
	"github.com/zncdatadev/dolphinscheduler-operator/pkg/resource"
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
	mergedCfg *dolphinv1alpha1.MasterRoleGroupSpec,
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
	core.MultiResourceReconciler[*dolphinv1alpha1.DolphinschedulerCluster, *dolphinv1alpha1.MasterRoleGroupSpec]
}

func (c *ConfigMapReconciler) Build(ctx context.Context) ([]core.ResourceBuilder, error) {
	return []core.ResourceBuilder{
		c.createEnvConfigMapReconciler(),
		c.createConfigConfigMapReconciler(),
	}, nil
}

// create env configmap
func (c *ConfigMapReconciler) createEnvConfigMapReconciler() core.ResourceBuilder {
	var generators []interface{}
	generators = append(generators, &common.EnvPropertiesGenerator{})
	var configOverrideHandler core.ConfigurationOverride
	if cfgOverride := c.MergedCfg.ConfigOverrides; cfgOverride != nil {
		configOverrideHandler = &EnvConfigmapOverride{EnvOverrideSpec: cfgOverride.Envs}
	}
	cm := resource.NewGeneralConfigMap(
		c.Scheme,
		c.Instance,
		c.Client,
		c.GroupName,
		c.Labels,
		c.MergedCfg,
		common.EnvsConfigMapName(c.Instance.GetName(), c.GroupName),
		generators,
		configOverrideHandler,
	)
	return cm
}

// crate config configmap
func (c *ConfigMapReconciler) createConfigConfigMapReconciler() core.ResourceBuilder {
	var generators []interface{}
	generators = append(generators, common.NewConfigPropertiesGenerator(c.Instance.Spec.ClusterConfigSpec.S3Bucket,
		c.Client, c.Instance.GetNamespace()))
	var configOverrideHandler core.ConfigurationOverride
	if cfgOverride := c.MergedCfg.ConfigOverrides; cfgOverride != nil {
		configOverrideHandler = &CommonPropertiesConfigmapOverride{CommonPropertiesOverrideSpec: cfgOverride.CommonProperties}
	}
	cm := resource.NewGeneralConfigMap(
		c.Scheme,
		c.Instance,
		c.Client,
		c.GroupName,
		c.Labels,
		c.MergedCfg,
		common.ConfigConfigMapName(c.Instance.GetName(), c.GroupName),
		generators,
		configOverrideHandler,
	)
	return cm
}

var _ core.ConfigurationOverride = &EnvConfigmapOverride{}
var _ core.ConfigurationOverride = &CommonPropertiesConfigmapOverride{}

// EnvConfigmapOverride env configmap
type EnvConfigmapOverride struct {
	EnvOverrideSpec map[string]string
}

func (e *EnvConfigmapOverride) ConfigurationOverride(obj client.Object) {
	if e.EnvOverrideSpec == nil {
		return
	}
	cm := obj.(*corev1.ConfigMap)
	origin := cm.Data
	resource.OverrideConfigmapEnvs(&origin, e.EnvOverrideSpec)
}

// CommonPropertiesConfigmapOverride common properties
type CommonPropertiesConfigmapOverride struct {
	CommonPropertiesOverrideSpec map[string]string
}

func (c *CommonPropertiesConfigmapOverride) ConfigurationOverride(obj client.Object) {
	cm := obj.(*corev1.ConfigMap)
	overridden := resource.OverrideConfigFileContent(cm.Data[dolphinv1alpha1.DolphinCommonPropertiesName],
		c.CommonPropertiesOverrideSpec, resource.Properties)
	cm.Data[dolphinv1alpha1.DolphinCommonPropertiesName] = overridden
}
