package cluster

import (
	"fmt"

	dolphinv1alpha1 "github.com/zncdatadev/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdatadev/dolphinscheduler-operator/internal/common"
	"github.com/zncdatadev/dolphinscheduler-operator/pkg/util"
	"github.com/zncdatadev/operator-go/pkg/builder"
	"github.com/zncdatadev/operator-go/pkg/client"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	opgoutil "github.com/zncdatadev/operator-go/pkg/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

const (
	ContainerDbInitJob util.ContainerComponent = "dolphinscheduler-db-init-job"
	ContainerWaitForDb util.ContainerComponent = "wait-for-database"
)

func NewJobInitScriptReconciler(
	client *client.Client,
	image *opgoutil.Image,
	clusterInfo reconciler.ClusterInfo,
	clusterConfigSpec *dolphinv1alpha1.ClusterConfigSpec,
) reconciler.ResourceReconciler[builder.JobBuilder] {

	options := builder.WorkloadOptions{
		Options: builder.Options{
			ClusterName:   clusterInfo.GetClusterName(),
			RoleName:      "",
			RoleGroupName: "",
			Labels:        clusterInfo.GetLabels(),
			Annotations:   clusterInfo.GetAnnotations(),
		},
	}
	b := builder.NewGenericJobBuilder(client, client.GetOwnerName(), image, options)
	b.SetRestPolicy(ptr.To(corev1.RestartPolicyNever))
	b.AddInitContainer(waitForDatabase(clusterConfigSpec.Database.ConnectionString))
	b.AddContainer(initDatabase(clusterConfigSpec.ZookeeperConfigMapName, image, clusterInfo))
	b.SetSecurityContext(1001, 1001, false)
	return reconciler.NewJob(client, client.GetOwnerName(), b)
}

func waitForDatabase(connectionString string) *corev1.Container {
	dbHost, _ := util.GetDatabaseHost(connectionString)
	return &corev1.Container{
		Name:  string(ContainerWaitForDb),
		Image: "busybox:1.30.1",
		Command: []string{
			"sh",
			"-xc",
			fmt.Sprintf("for i in $(seq 1 180); do nc -z -w3 %s 5432 && exit 0 || sleep 5; done; exit 1", dbHost),
		},
	}
}

func initDatabase(zookeeperConfigMapName string, image *opgoutil.Image, clusterInfo reconciler.ClusterInfo) *corev1.Container {
	rolegroup := reconciler.RoleGroupInfo{
		RoleInfo: reconciler.RoleInfo{ClusterInfo: clusterInfo},
	}
	return common.NewContainerBuilder(ContainerDbInitJob, image, zookeeperConfigMapName, &rolegroup, nil).
		ResetVolumeMounts().
		ResetCommandArgs("chmod +x tools/bin/upgrade-schema.sh && tools/bin/upgrade-schema.sh").Build()
}
