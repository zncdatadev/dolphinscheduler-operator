package cluster

import (
	"context"

	"github.com/zncdatadev/operator-go/pkg/client"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	"github.com/zncdatadev/operator-go/pkg/util"

	dolphinv1alpha1 "github.com/zncdatadev/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdatadev/dolphinscheduler-operator/internal/common"
	"github.com/zncdatadev/dolphinscheduler-operator/internal/controller/alerter"
	"github.com/zncdatadev/dolphinscheduler-operator/internal/controller/api"
	"github.com/zncdatadev/dolphinscheduler-operator/internal/controller/master"
	"github.com/zncdatadev/dolphinscheduler-operator/internal/controller/worker"
	"github.com/zncdatadev/dolphinscheduler-operator/internal/util/version"
	pkgutil "github.com/zncdatadev/dolphinscheduler-operator/pkg/util"
)

var _ reconciler.Reconciler = &Reconciler{}

type Reconciler struct {
	reconciler.BaseCluster[*dolphinv1alpha1.DolphinschedulerClusterSpec]
	ClusterConfig *dolphinv1alpha1.ClusterConfigSpec
}

func NewClusterReconciler(
	client *client.Client,
	clusterInfo reconciler.ClusterInfo,
	spec *dolphinv1alpha1.DolphinschedulerClusterSpec,
) *Reconciler {
	return &Reconciler{
		BaseCluster: *reconciler.NewBaseCluster(
			client,
			clusterInfo,
			spec.ClusterOperationSpec,
			spec,
		),
		ClusterConfig: spec.ClusterConfig,
	}
}

func (r *Reconciler) GetImage() *util.Image {
	image := util.NewImage(
		dolphinv1alpha1.DefaultProductName,
		version.BuildVersion,
		dolphinv1alpha1.DefaultProductVersion,
		func(options *util.ImageOptions) {
			options.Custom = r.Spec.Image.Custom
			options.Repo = r.Spec.Image.Repo
			options.PullPolicy = *r.Spec.Image.PullPolicy
		},
	)

	if r.Spec.Image.KubedoopVersion != "" {
		image.KubedoopVersion = r.Spec.Image.KubedoopVersion
	}

	return image
}

// RegisterResources implements reconciler.ClusterReconciler
// register resources for cluster
// include:
//   - service account, role, rolebinding
//   - env configmap
//   - roles: alerter, api, master,worker server
//   - init-db job
func (r *Reconciler) RegisterResources(ctx context.Context) error {
	client := r.GetClient()
	clusterLables := r.ClusterInfo.GetLabels()
	annotations := r.ClusterInfo.GetAnnotations()

	// Register rbac resources
	// register service account, role, rolebinding
	sa := NewServiceAccountReconciler(*r.Client, clusterLables, annotations)
	role := NewRoleReconciler(*r.Client, clusterLables, annotations)
	rb := NewRoleBindingReconciler(*r.Client, clusterLables, annotations)
	r.AddResource(sa)
	r.AddResource(role)
	r.AddResource(rb)

	// Register init db job
	initDbJob := NewJobInitScriptReconciler(client, r.GetImage(), r.ClusterInfo, r.ClusterConfig)
	r.AddResource(initDbJob)
	// Register env configmap

	// Register roles:
	// alerter, api, master,worker server
	roleInfo := func(role pkgutil.Role) reconciler.RoleInfo {
		return reconciler.RoleInfo{ClusterInfo: r.ClusterInfo, RoleName: string(role)}
	}
	masterServerRole := master.NewMasterRole(client, r.GetImage(), r.ClusterConfig, r.ClusterOperation, r.Spec.Master, roleInfo(common.Master))
	alerterServerRole := alerter.NewAlerterRole(client, r.GetImage(), r.ClusterConfig, r.ClusterOperation, r.Spec.Alerter, roleInfo(common.Alerter))
	apiServerRole := api.NewApierRole(client, r.GetImage(), r.ClusterConfig, r.ClusterOperation, r.Spec.Api, roleInfo(common.Api))
	workerServerRole := worker.NewWorkerRole(client, r.GetImage(), r.ClusterConfig, r.ClusterOperation, r.Spec.Worker, roleInfo(common.Worker))
	if err := masterServerRole.RegisterResources(ctx); err != nil {
		return err
	}
	if err := alerterServerRole.RegisterResources(ctx); err != nil {
		return err
	}
	if err := apiServerRole.RegisterResources(ctx); err != nil {
		return err
	}
	if err := workerServerRole.RegisterResources(ctx); err != nil {
		return err
	}
	r.AddResource(masterServerRole)
	r.AddResource(alerterServerRole)
	r.AddResource(apiServerRole)
	r.AddResource(workerServerRole)

	return nil
}
