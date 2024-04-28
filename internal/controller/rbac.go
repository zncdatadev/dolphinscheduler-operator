package controller

import (
	dolphinv1alpha1 "github.com/zncdata-labs/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdata-labs/dolphinscheduler-operator/internal/common"
	"github.com/zncdata-labs/dolphinscheduler-operator/pkg/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var serviceAccountName = func(instanceName string) string { return common.CreateServiceAccountName(instanceName) }
var roleName = "dolphinscheduler-role"
var roleBindingName = "dolphinscheduler-rolebinding"

// NewServiceAccount new a ServiceAccountReconciler
func NewServiceAccount(
	scheme *runtime.Scheme,
	instance *dolphinv1alpha1.DolphinschedulerCluster,
	client client.Client,
	mergedLabels map[string]string,
	mergedCfg any,
) *resource.GenericServiceAccountReconciler[*dolphinv1alpha1.DolphinschedulerCluster, any] {
	return resource.NewServiceAccount[*dolphinv1alpha1.DolphinschedulerCluster](scheme, instance, client, mergedLabels, mergedCfg,
		serviceAccountName(instance.GetName()), instance.GetNamespace())
}

// NewRole new a ClusterRoleReconciler
func NewRole(
	scheme *runtime.Scheme,
	instance *dolphinv1alpha1.DolphinschedulerCluster,
	client client.Client,
	mergedLabels map[string]string,
	mergedCfg any,
) *resource.GenericRoleReconciler[*dolphinv1alpha1.DolphinschedulerCluster, any] {
	return resource.NewRole[*dolphinv1alpha1.DolphinschedulerCluster](
		scheme,
		instance,
		client,
		"",
		mergedLabels,
		mergedCfg,
		resource.RbacRole,
		roleName,
		[]resource.VerbType{resource.Get, resource.List, resource.Watch},
		[]string{""},
		[]resource.ResourceType{resource.ConfigMaps},
		instance.Namespace,
	)
}

// NewRoleBinding new a ClusterRoleBindingReconciler
func NewRoleBinding(
	scheme *runtime.Scheme,
	instance *dolphinv1alpha1.DolphinschedulerCluster,
	client client.Client,
	mergedLabels map[string]string,
	mergedCfg any,
) *resource.GenericRoleBindingReconciler[*dolphinv1alpha1.DolphinschedulerCluster, any] {
	return resource.NewRoleBinding[*dolphinv1alpha1.DolphinschedulerCluster](
		scheme,
		instance,
		client,
		"",
		mergedLabels,
		mergedCfg,

		"",
		resource.RoleBinding,
		roleBindingName,
		roleName,
		serviceAccountName(instance.GetName()),
		instance.GetNamespace(),
	)
}
