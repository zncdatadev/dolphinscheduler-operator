package api

import (
	"context"

	dolphinv1alpha1 "github.com/zncdatadev/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdatadev/dolphinscheduler-operator/internal/common"
	"github.com/zncdatadev/dolphinscheduler-operator/pkg/util"
	commonsv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/client"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	opgoutil "github.com/zncdatadev/operator-go/pkg/util"
)

func NewApierRole(
	client *client.Client,
	image *opgoutil.Image,
	clusterConfigSpec *dolphinv1alpha1.ClusterConfigSpec,
	clusterOperation *commonsv1alpha1.ClusterOperationSpec,
	apiRoleSpec *dolphinv1alpha1.RoleSpec,
	roleInfo reconciler.RoleInfo) *common.RoleReconciler {

	apiRoleResourcesReconcilersBuilder := &ApierRoleResourceReconcilerBuilder{
		client:           client,
		clusterOperation: clusterOperation,
		image:            image,
		zkConfigMapName:  clusterConfigSpec.ZookeeperConfigMapName,
	}
	return common.NewRoleReconciler(client, roleInfo, clusterOperation, clusterConfigSpec, image,
		*apiRoleSpec, apiRoleResourcesReconcilersBuilder)
}

var _ common.RoleResourceReconcilersBuilder = &ApierRoleResourceReconcilerBuilder{}

type ApierRoleResourceReconcilerBuilder struct {
	client           *client.Client
	clusterOperation *commonsv1alpha1.ClusterOperationSpec
	image            *opgoutil.Image
	zkConfigMapName  string
}

// Buile implements common.RoleReconcilerBuilder.
// api server role has resources below:
// - deployment
// - service
func (a *ApierRoleResourceReconcilerBuilder) ResourceReconcilers(ctx context.Context, roleGroupInfo *reconciler.RoleGroupInfo,
	mergedCfg *dolphinv1alpha1.RoleGroupSpec) []reconciler.Reconciler {
	var reconcilers []reconciler.Reconciler

	//Configmap
	workerConfigMap := common.NewConfigMapReconciler(ctx, a.client, roleGroupInfo, MainContainerName, mergedCfg)
	reconcilers = append(reconcilers, workerConfigMap)

	//deployment
	containerBuilder := common.NewContainerBuilder(MainContainerName, a.image, a.zkConfigMapName, roleGroupInfo).WithCommandArgs().WithEnvFrom().
		WithPorts(util.SortedMap{
			dolphinv1alpha1.ApiPortName:       dolphinv1alpha1.ApiPort,
			dolphinv1alpha1.ApiPythonPortName: dolphinv1alpha1.ApiPythonPort,
		}).
		WithEnvs(util.SortedMap{"JAVA_OPTS": "-Xms512m -Xmx512m -Xmn256m"}).
		WithReadinessAndLivenessProbe(dolphinv1alpha1.ApiPort).
		WithCommandArgs().
		WithVolumeMounts(nil)
	dep := common.CreateDeploymentReconciler(containerBuilder, ctx, a.client, a.image, a.clusterOperation, roleGroupInfo, mergedCfg, a.zkConfigMapName)
	reconcilers = append(reconcilers, dep)

	//svc
	svc := common.NewServiceReconciler(a.client, common.RoleGroupServiceName(roleGroupInfo), false, nil, map[string]int32{
		dolphinv1alpha1.ApiPortName:       dolphinv1alpha1.ApiPort,
		dolphinv1alpha1.ApiPythonPortName: dolphinv1alpha1.ApiPythonPort,
	}, roleGroupInfo.GetLabels(), roleGroupInfo.GetAnnotations())
	reconcilers = append(reconcilers, svc)

	//ingress , deprecated, use node port service instead
	// ingress := NewIngressReconciler(a.client, roleGroupInfo)
	// reconcilers = append(reconcilers, ingress)

	return reconcilers
}
