package master

import (
	"context"
	dolphinv1alpha1 "github.com/zncdata-labs/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdata-labs/dolphinscheduler-operator/internal/common"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/core"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/resource"
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
	cm := resource.NewGeneralConfigMap(
		c.Scheme,
		c.Instance,
		c.Client,
		c.GroupName,
		c.Labels,
		c.MergedCfg,
		common.EnvsConfigMapName(c.Instance.GetName(), c.GroupName),
		generators,
		nil, // todo
	)
	return cm
}

// crate config configmap
func (c *ConfigMapReconciler) createConfigConfigMapReconciler() core.ResourceBuilder {
	var generators []interface{}
	generators = append(generators, common.NewConfigPropertiesGenerator(c.Instance.Spec.ClusterConfigSpec.S3Bucket,
		c.Client, c.Instance.GetNamespace()))
	cm := resource.NewGeneralConfigMap(
		c.Scheme,
		c.Instance,
		c.Client,
		c.GroupName,
		c.Labels,
		c.MergedCfg,
		common.ConfigConfigMapName(c.Instance.GetName(), c.GroupName),
		generators,
		nil, // todo
	)
	return cm
}
