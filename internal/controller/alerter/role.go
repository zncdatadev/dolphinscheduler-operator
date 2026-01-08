package alerter

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

func NewAlerterRole(
	client *client.Client,
	image *opgoutil.Image,
	clusterConfigSpec *dolphinv1alpha1.ClusterConfigSpec,
	clusterOperation *commonsv1alpha1.ClusterOperationSpec,
	alertRoleSpec *dolphinv1alpha1.RoleSpec,
	roleInfo reconciler.RoleInfo) *common.RoleReconciler {
	alerterRoleResourcesReconcilersBuilder := &AlerterRoleResourceReconcilerBuilder{
		client:           client,
		clusterOperation: clusterOperation,
		image:            image,
		zkConfigMapName:  clusterConfigSpec.ZookeeperConfigMapName,
	}
	return common.NewRoleReconciler(client, roleInfo, clusterOperation, clusterConfigSpec, image, *alertRoleSpec, alerterRoleResourcesReconcilersBuilder)
}

var _ common.RoleResourceReconcilersBuilder = &AlerterRoleResourceReconcilerBuilder{}

type AlerterRoleResourceReconcilerBuilder struct {
	client           *client.Client
	clusterOperation *commonsv1alpha1.ClusterOperationSpec
	image            *opgoutil.Image
	zkConfigMapName  string
}

// Buile implements common.RoleReconcilerBuilder.
// alerter role has resources below:
// - deployment
// - service
func (a *AlerterRoleResourceReconcilerBuilder) ResourceReconcilers(
	ctx context.Context,
	replicas *int32,
	roleGroupInfo *reconciler.RoleGroupInfo,
	overrides *commonsv1alpha1.OverridesSpec,
	roleGroupConfig *commonsv1alpha1.RoleGroupConfigSpec,
) []reconciler.Reconciler {
	reconcilers := make([]reconciler.Reconciler, 0, 3)

	// Configmap
	workerConfigMap := common.NewConfigMapReconciler(
		ctx,
		a.client,
		roleGroupInfo,
		MainContainerName,
		overrides,
		roleGroupConfig,
	)
	reconcilers = append(reconcilers, workerConfigMap)

	// deployment
	containerBuilder := common.NewContainerBuilder(
		MainContainerName,
		a.image,
		a.zkConfigMapName,
		roleGroupInfo,
		roleGroupConfig,
	).
		CommonCommandArgs().
		WithPorts(util.SortedMap{
			dolphinv1alpha1.AlerterPortName:       dolphinv1alpha1.AlerterPort,
			dolphinv1alpha1.AlerterActualPortName: dolphinv1alpha1.AlerterActualPort,
		}).
		WithEnvs(util.SortedMap{"JAVA_OPTS": "-Xms512m -Xmx512m -Xmn256m"}).
		WithReadinessAndLivenessProbe(dolphinv1alpha1.AlerterActualPort).
		CommonCommandArgs().
		WithVolumeMounts(nil)
	dep := common.CreateDeploymentReconciler(
		containerBuilder,
		ctx,
		a.client,
		a.image,
		replicas,
		a.clusterOperation,
		roleGroupInfo,
		overrides,
		roleGroupConfig,
		a.zkConfigMapName,
		nil,
	)
	reconcilers = append(reconcilers, dep)

	// svc
	svc := common.NewServiceReconciler(
		a.client,
		common.RoleGroupServiceName(roleGroupInfo),
		false,
		nil,
		map[string]int32{
			dolphinv1alpha1.AlerterPortName:       dolphinv1alpha1.AlerterPort,
			dolphinv1alpha1.AlerterActualPortName: dolphinv1alpha1.AlerterActualPort,
		},
		roleGroupInfo.GetLabels(),
		roleGroupInfo.GetAnnotations(),
	)
	reconcilers = append(reconcilers, svc)

	return reconcilers
}
