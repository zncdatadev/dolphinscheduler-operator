package master

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

func NewMasterRole(
	client *client.Client,
	image *opgoutil.Image,
	clusterConfigSpec *dolphinv1alpha1.ClusterConfigSpec,
	clusterOperation *commonsv1alpha1.ClusterOperationSpec,
	apiRoleSpec *dolphinv1alpha1.RoleSpec,
	roleInfo reconciler.RoleInfo) *common.RoleReconciler {

	apiRoleResourcesReconcilersBuilder := &MasterRoleResourceReconcilerBuilder{
		client:           client,
		clusterOperation: clusterOperation,
		image:            image,
		zkConfigMapName:  clusterConfigSpec.ZookeeperConfigMapName,
	}
	return common.NewRoleReconciler(client, roleInfo, clusterOperation, clusterConfigSpec, image,
		*apiRoleSpec, apiRoleResourcesReconcilersBuilder)
}

var _ common.RoleResourceReconcilersBuilder = &MasterRoleResourceReconcilerBuilder{}

type MasterRoleResourceReconcilerBuilder struct {
	client           *client.Client
	clusterOperation *commonsv1alpha1.ClusterOperationSpec
	image            *opgoutil.Image
	zkConfigMapName  string
}

// Buile implements common.RoleReconcilerBuilder.
// api server role has resources below:
// - deployment
// - service
func (a *MasterRoleResourceReconcilerBuilder) ResourceReconcilers(ctx context.Context, roleGroupInfo *reconciler.RoleGroupInfo,
	mergedCfg *dolphinv1alpha1.RoleGroupSpec) []reconciler.Reconciler {
	var reconcilers []reconciler.Reconciler

	// Configmap
	masterConfigMap := common.NewConfigMapReconciler(ctx, a.client, roleGroupInfo, MainContainerName, mergedCfg)
	reconcilers = append(reconcilers, masterConfigMap)

	//env from configmap
	envFromConfigMap := common.NewEnvConfigMapReconciler(ctx, a.client, mergedCfg, roleGroupInfo)
	reconcilers = append(reconcilers, envFromConfigMap)

	//statefulset
	containerBuilder := common.NewContainerBuilder(MainContainerName, a.image, a.zkConfigMapName, roleGroupInfo).WithCommandArgs().WithEnvFrom().
		WithPorts(util.SortedMap{
			dolphinv1alpha1.MasterPortName:       dolphinv1alpha1.MasterPort,
			dolphinv1alpha1.MasterActualPortName: dolphinv1alpha1.MasterActualPort,
		}).
		WithEnvs(util.SortedMap{
			"JAVA_OPTS":                                    "-Xms1g -Xmx1g -Xmn512m",
			"MASTER_DISPATCH_TASK_NUM":                     "3",
			"MASTER_EXEC_TASK_NUM":                         "20",
			"MASTER_EXEC_THREADS":                          "100",
			"MASTER_FAILOVER_INTERVAL":                     "10m",
			"MASTER_HEARTBEAT_ERROR_THRESHOLD":             "5",
			"MASTER_HOST_SELECTOR":                         "LowerWeight",
			"MASTER_KILL_APPLICATION_WHEN_HANDLE_FAILOVER": "true",
			"MASTER_MAX_HEARTBEAT_INTERVAL":                "10s",
			"MASTER_SERVER_LOAD_PROTECTION_ENABLED":        "false",
			"MASTER_SERVER_LOAD_PROTECTION_MAX_DISK_USAGE_PERCENTAGE_THRESHOLDS":          "0.7",
			"MASTER_SERVER_LOAD_PROTECTION_MAX_JVM_CPU_USAGE_PERCENTAGE_THRESHOLDS":       "0.7",
			"MASTER_SERVER_LOAD_PROTECTION_MAX_SYSTEM_CPU_USAGE_PERCENTAGE_THRESHOLDS":    "0.7",
			"MASTER_SERVER_LOAD_PROTECTION_MAX_SYSTEM_MEMORY_USAGE_PERCENTAGE_THRESHOLDS": "0.7",
			"MASTER_STATE_WHEEL_INTERVAL":                                                 "5s",
			"MASTER_TASK_COMMIT_INTERVAL":                                                 "1s",
			"MASTER_TASK_COMMIT_RETRYTIMES":                                               "5",
		}).
		WithReadinessAndLivenessProbe(dolphinv1alpha1.MasterActualPort).
		WithCommandArgs().
		WithVolumeMounts(nil)
	sts := common.CreateStatefulSetReconciler(containerBuilder, ctx, a.client, a.image, a.clusterOperation, roleGroupInfo, mergedCfg,
		a.zkConfigMapName, "")
	reconcilers = append(reconcilers, sts)

	//svc
	svc := common.NewServiceReconciler(a.client, common.RoleGroupServiceName(roleGroupInfo), true, nil, map[string]int32{
		dolphinv1alpha1.MasterPortName:       dolphinv1alpha1.MasterPort,
		dolphinv1alpha1.MasterActualPortName: dolphinv1alpha1.MasterActualPort,
	}, roleGroupInfo.GetLabels(), roleGroupInfo.GetAnnotations())
	reconcilers = append(reconcilers, svc)

	return reconcilers
}
