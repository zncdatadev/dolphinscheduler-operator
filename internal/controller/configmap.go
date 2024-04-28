package controller

import (
	dolphinv1alpha1 "github.com/zncdata-labs/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdata-labs/dolphinscheduler-operator/internal/common"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewConfigMap(
	scheme *runtime.Scheme,
	instance *dolphinv1alpha1.DolphinschedulerCluster,
	client client.Client,
	labels map[string]string,
	mergedCfg any,
) *resource.GeneralConfigMapReconciler[*dolphinv1alpha1.DolphinschedulerCluster, any] {
	configMapName := common.EnvsConfigMapName(instance.GetName(), "")
	generators := []interface{}{
		&common.EnvPropertiesGenerator{},
	}
	return resource.NewGeneralConfigMap(scheme, instance, client, "", labels, mergedCfg, configMapName, generators, nil)
}
