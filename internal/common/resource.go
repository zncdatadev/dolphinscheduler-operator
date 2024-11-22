package common

import (
	"context"

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
	replicas *int32,
	clusterOperation *commonsv1alpha1.ClusterOperationSpec,
	roleGroupInfo *reconciler.RoleGroupInfo,
	overrides *commonsv1alpha1.OverridesSpec,
	roleGroupConfig *commonsv1alpha1.RoleGroupConfigSpec,
	zkConfigMapName string,
	volumes []corev1.Volume) reconciler.ResourceReconciler[builder.DeploymentBuilder] {
	container := contanerBuilder.Build()
	// stopped
	stopped := false
	if clusterOperation != nil && clusterOperation.Stopped {
		stopped = true
	}
	return NewDeploymentReconciler(
		ctx,
		client,
		stopped,
		image,
		replicas,
		roleGroupInfo,
		overrides,
		roleGroupConfig,
		[]corev1.Container{*container},
		volumes)
	// return NewDeploymentReconciler(ctx, mergedCfg, client, stopped, image, options, roleGroupInfo, []corev1.Container{*container}, volumes)
}
func CreateStatefulSetReconciler(
	containerBuilder *ContainerBuilder,
	ctx context.Context,
	client *client.Client,
	image *opgoutil.Image,
	replicas *int32,
	clusterOperation *commonsv1alpha1.ClusterOperationSpec,
	roleGroupInfo *reconciler.RoleGroupInfo,
	overrides *commonsv1alpha1.OverridesSpec,
	roleGroupConfig *commonsv1alpha1.RoleGroupConfigSpec,
	zkConfigMapName string,
	pvcName string) reconciler.ResourceReconciler[builder.StatefulSetBuilder] {
	container := containerBuilder.Build()
	// stopped
	stopped := false
	if clusterOperation != nil && clusterOperation.Stopped {
		stopped = true
	}
	var storageSize *resource.Quantity
	resource := roleGroupConfig.Resources
	if resource != nil && resource.Storage != nil {
		storageSize = &roleGroupConfig.Resources.Storage.Capacity
	}
	return NewStatefulSetReconciler(
		ctx,
		client,
		stopped,
		image,
		replicas,
		roleGroupInfo,
		overrides,
		roleGroupConfig,
		[]corev1.Container{*container},
		pvcName,
		storageSize)
}
