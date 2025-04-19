package common

import (
	"context"

	"github.com/zncdatadev/operator-go/pkg/client"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	opgoutil "github.com/zncdatadev/operator-go/pkg/util"
	ctrl "sigs.k8s.io/controller-runtime"

	dolphinv1alpha1 "github.com/zncdatadev/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdatadev/dolphinscheduler-operator/pkg/util"
	commonsv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
)

var (
	logger = ctrl.Log.WithName("controller").WithName("zk-server")
)

func NewRoleReconciler(
	client *client.Client,
	roleInfo reconciler.RoleInfo,
	clusterOperation *commonsv1alpha1.ClusterOperationSpec,
	clusterConfig *dolphinv1alpha1.ClusterConfigSpec,
	image *opgoutil.Image,
	spec dolphinv1alpha1.RoleSpec,
	roleResourceReconcilersBuilder RoleResourceReconcilersBuilder,
) *RoleReconciler {
	stopped := clusterOperation != nil && clusterOperation.Stopped

	return &RoleReconciler{
		BaseRoleReconciler: *reconciler.NewBaseRoleReconciler(
			client,
			stopped,
			roleInfo,
			spec,
		),
		Image:                          image,
		ClusterConfig:                  clusterConfig,
		roleResourceReconcilersBuilder: roleResourceReconcilersBuilder,
	}
}

var _ reconciler.RoleReconciler = &RoleReconciler{}

type RoleReconciler struct {
	reconciler.BaseRoleReconciler[dolphinv1alpha1.RoleSpec]
	ClusterConfig *dolphinv1alpha1.ClusterConfigSpec
	Image         *opgoutil.Image

	roleResourceReconcilersBuilder RoleResourceReconcilersBuilder
}

type RoleResourceReconcilersBuilder interface {
	ResourceReconcilers(
		ctx context.Context,
		replicas *int32,
		info *reconciler.RoleGroupInfo,
		overrides *commonsv1alpha1.OverridesSpec,
		roleGroupConfig *commonsv1alpha1.RoleGroupConfigSpec,
	) []reconciler.Reconciler
}

func (r *RoleReconciler) RegisterResources(ctx context.Context) error {
	for name, roleGroup := range r.Spec.RoleGroups {
		mergedConfig, err := opgoutil.MergeObject(r.Spec.Config, roleGroup.Config)
		if err != nil {
			return err
		}
		overrides, err := opgoutil.MergeObject(r.Spec.OverridesSpec, roleGroup.OverridesSpec)
		if err != nil {
			return err
		}
		// merge default config to the provided config
		defaultConfig := DefaultConfig(util.Role(r.RoleInfo.RoleName), r.Client.GetOwnerName())
		if mergedConfig == nil {
			mergedConfig = &dolphinv1alpha1.ConfigSpec{}
		}
		if overrides == nil {
			overrides = &commonsv1alpha1.OverridesSpec{}
		}
		err = defaultConfig.MergeDefaultConfig(overrides, mergedConfig)
		if err != nil {
			return err
		}

		info := &reconciler.RoleGroupInfo{
			RoleInfo:      r.RoleInfo,
			RoleGroupName: name,
		}
		reconcilers, err := r.RegisterResourceWithRoleGroup(
			ctx,
			roleGroup.Replicas,
			info,
			overrides,
			mergedConfig.RoleGroupConfigSpec,
		)
		if err != nil {
			return err
		}

		for _, reconciler := range reconcilers {
			r.AddResource(reconciler)
			logger.Info("registered resource", "role", r.GetName(), "roleGroup", name, "reconciler", reconciler.GetName())
		}
	}
	return nil
}

func (r *RoleReconciler) RegisterResourceWithRoleGroup(
	ctx context.Context,
	replicas *int32,
	info *reconciler.RoleGroupInfo,
	overrides *commonsv1alpha1.OverridesSpec,
	roleGroupConfig *commonsv1alpha1.RoleGroupConfigSpec,
) ([]reconciler.Reconciler, error) {
	return r.roleResourceReconcilersBuilder.ResourceReconcilers(ctx, replicas, info, overrides, roleGroupConfig), nil
}
