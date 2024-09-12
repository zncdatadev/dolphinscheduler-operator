package worker

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

func NewWorkerRole(
	client *client.Client,
	image *opgoutil.Image,
	clusterConfigSpec *dolphinv1alpha1.ClusterConfigSpec,
	clusterOperation *commonsv1alpha1.ClusterOperationSpec,
	apiRoleSpec *dolphinv1alpha1.RoleSpec,
	roleInfo reconciler.RoleInfo) *common.RoleReconciler {

	apiRoleResourcesReconcilersBuilder := &WorkerRoleResourceReconcilerBuilder{
		client:           client,
		clusterOperation: clusterOperation,
		image:            image,
		zkConfigMapName:  clusterConfigSpec.ZookeeperConfigMapName,
	}
	return common.NewRoleReconciler(client, roleInfo, clusterOperation, clusterConfigSpec, image,
		*apiRoleSpec, apiRoleResourcesReconcilersBuilder)
}

var _ common.RoleResourceReconcilersBuilder = &WorkerRoleResourceReconcilerBuilder{}

type WorkerRoleResourceReconcilerBuilder struct {
	client           *client.Client
	clusterOperation *commonsv1alpha1.ClusterOperationSpec
	image            *opgoutil.Image
	zkConfigMapName  string
}

// Buile implements common.RoleReconcilerBuilder.
// api server role has resources below:
// - deployment
// - service
func (a *WorkerRoleResourceReconcilerBuilder) ResourceReconcilers(ctx context.Context, roleGroupInfo *reconciler.RoleGroupInfo,
	mergedCfg *dolphinv1alpha1.RoleGroupSpec) []reconciler.Reconciler {
	var reconcilers []reconciler.Reconciler

	//common.properties, logback.xml Configmap
	workerConfigMap := common.NewConfigMapReconciler(ctx, a.client, roleGroupInfo, MainContainerName, mergedCfg)
	reconcilers = append(reconcilers, workerConfigMap)

	//statefulset
	containerBuilder := common.NewContainerBuilder(MainContainerName, a.image, a.zkConfigMapName, roleGroupInfo).WithCommandArgs().WithEnvFrom().
		WithPorts(util.SortedMap{
			dolphinv1alpha1.WorkerPortName:       dolphinv1alpha1.WorkerPort,
			dolphinv1alpha1.WorkerActualPortName: dolphinv1alpha1.WorkerActualPort,
		}).
		WithEnvs(util.SortedMap{
			"DEFAULT_TENANT_ENABLED":                "false",
			"WORKER_EXEC_THREADS":                   "100",
			"WORKER_HOST_WEIGHT":                    "100",
			"WORKER_MAX_HEARTBEAT_INTERVAL":         "10s",
			"WORKER_SERVER_LOAD_PROTECTION_ENABLED": "false",
			"WORKER_SERVER_LOAD_PROTECTION_MAX_DISK_USAGE_PERCENTAGE_THRESHOLDS":          "0.7",
			"WORKER_SERVER_LOAD_PROTECTION_MAX_JVM_CPU_USAGE_PERCENTAGE_THRESHOLDS":       "0.7",
			"WORKER_SERVER_LOAD_PROTECTION_MAX_SYSTEM_CPU_USAGE_PERCENTAGE_THRESHOLDS":    "0.7",
			"WORKER_SERVER_LOAD_PROTECTION_MAX_SYSTEM_MEMORY_USAGE_PERCENTAGE_THRESHOLDS": "0.7",
			"WORKER_TENANT_CONFIG_AUTO_CREATE_TENANT_ENABLED":                             "true",
			"WORKER_TENANT_CONFIG_DISTRIBUTED_TENANT":                                     "false",
		}).
		WithReadinessAndLivenessProbe(dolphinv1alpha1.WorkerActualPort).
		WithCommandArgs().
		WithVolumeMounts(nil)
	dep := common.CreateStatefulSetReconciler(containerBuilder, ctx, a.client, a.image, a.clusterOperation, roleGroupInfo, mergedCfg,
		a.zkConfigMapName, dolphinv1alpha1.WorkerDataVolumeName)
	reconcilers = append(reconcilers, dep)

	//svc
	svc := common.NewServiceReconciler(a.client, common.RoleGroupServiceName(roleGroupInfo), true, nil, map[string]int32{
		dolphinv1alpha1.WorkerPortName:       dolphinv1alpha1.WorkerPort,
		dolphinv1alpha1.WorkerActualPortName: dolphinv1alpha1.WorkerActualPort,
	}, roleGroupInfo.GetLabels(), roleGroupInfo.GetAnnotations())
	reconcilers = append(reconcilers, svc)

	return reconcilers
}
