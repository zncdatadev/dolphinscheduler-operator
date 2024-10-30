package common

import (
	"context"

	dolphinv1alpha1 "github.com/zncdatadev/dolphinscheduler-operator/api/v1alpha1"
	commonsv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/builder"
	"github.com/zncdatadev/operator-go/pkg/client"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	opgoutil "github.com/zncdatadev/operator-go/pkg/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func CreateDeploymentReconciler(
	contanerBuilder *ContainerBuilder,
	ctx context.Context,
	client *client.Client,
	image *opgoutil.Image,
	clusterOperation *commonsv1alpha1.ClusterOperationSpec,
	roleGroupInfo *reconciler.RoleGroupInfo,
	mergedCfg *dolphinv1alpha1.RoleGroupSpec,
	zkConfigMapName string, volumes []corev1.Volume) reconciler.ResourceReconciler[builder.DeploymentBuilder] {
	container := contanerBuilder.Build()
	// stopped
	stopped := false
	if clusterOperation != nil && clusterOperation.Stopped {
		stopped = true
	}
	// workload option
	options := builder.WorkloadOptions{
		Options: builder.Options{
			ClusterName:   roleGroupInfo.GetClusterName(),
			RoleName:      roleGroupInfo.GetRoleName(),
			RoleGroupName: roleGroupInfo.GetGroupName(),
			Labels:        roleGroupInfo.GetLabels(),
			Annotations:   roleGroupInfo.GetAnnotations(),
		},
		// PodOverrides:     mergedCfg.PodOverrides,
		CommandOverrides: mergedCfg.CliOverrides,
		EnvOverrides:     mergedCfg.EnvOverrides,
	}
	return NewDeploymentReconciler(ctx, mergedCfg, client, stopped, image, options, roleGroupInfo, []corev1.Container{*container}, volumes)
}
func CreateStatefulSetReconciler(
	containerBuilder *ContainerBuilder,
	ctx context.Context,
	client *client.Client,
	image *opgoutil.Image,
	clusterOperation *commonsv1alpha1.ClusterOperationSpec,
	roleGroupInfo *reconciler.RoleGroupInfo,
	mergedCfg *dolphinv1alpha1.RoleGroupSpec,
	zkConfigMapName string, pvcName string) reconciler.ResourceReconciler[builder.StatefulSetBuilder] {
	container := containerBuilder.Build()
	// stopped
	stopped := false
	if clusterOperation != nil && clusterOperation.Stopped {
		stopped = true
	}
	// workload option
	options := builder.WorkloadOptions{
		Options: builder.Options{
			ClusterName:   roleGroupInfo.GetClusterName(),
			RoleName:      roleGroupInfo.GetRoleName(),
			RoleGroupName: roleGroupInfo.GetGroupName(),
			Labels:        roleGroupInfo.GetLabels(),
			Annotations:   roleGroupInfo.GetAnnotations(),
		},
		// PodOverrides:     mergedCfg.PodOverrides,
		CommandOverrides: mergedCfg.CliOverrides,
		EnvOverrides:     mergedCfg.EnvOverrides,
	}

	var storageSize *resource.Quantity
	resource := mergedCfg.Config.Resources
	if resource != nil && resource.Storage != nil {
		storageSize = &mergedCfg.Config.Resources.Storage.Capacity
	}
	return NewStatefulSetReconciler(ctx, mergedCfg, client, stopped, image, options, roleGroupInfo, []corev1.Container{*container}, pvcName, storageSize)
}
