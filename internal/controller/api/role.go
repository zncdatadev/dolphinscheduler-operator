package api

import (
	"context"
	"strings"

	dolphinv1alpha1 "github.com/zncdatadev/dolphinscheduler-operator/api/v1alpha1"
	"github.com/zncdatadev/dolphinscheduler-operator/internal/common"
	"github.com/zncdatadev/dolphinscheduler-operator/internal/security"
	"github.com/zncdatadev/dolphinscheduler-operator/pkg/util"
	commonsv1alpha1 "github.com/zncdatadev/operator-go/pkg/apis/commons/v1alpha1"
	"github.com/zncdatadev/operator-go/pkg/client"
	"github.com/zncdatadev/operator-go/pkg/reconciler"
	opgoutil "github.com/zncdatadev/operator-go/pkg/util"
	corev1 "k8s.io/api/core/v1"
)

func NewApierRole(
	client *client.Client,
	image *opgoutil.Image,
	clusterConfigSpec *dolphinv1alpha1.ClusterConfigSpec,
	clusterOperation *commonsv1alpha1.ClusterOperationSpec,
	apiRoleSpec *dolphinv1alpha1.RoleSpec,
	roleInfo reconciler.RoleInfo) *common.RoleReconciler {

	apiRoleResourcesReconcilersBuilder := &ApierRoleResourceReconcilerBuilder{
		client:             client,
		clusterOperation:   clusterOperation,
		image:              image,
		zkConfigMapName:    clusterConfigSpec.ZookeeperConfigMapName,
		authenticationSpec: clusterConfigSpec.Authentication,
	}
	return common.NewRoleReconciler(client, roleInfo, clusterOperation, clusterConfigSpec, image,
		*apiRoleSpec, apiRoleResourcesReconcilersBuilder)
}

var _ common.RoleResourceReconcilersBuilder = &ApierRoleResourceReconcilerBuilder{}

type ApierRoleResourceReconcilerBuilder struct {
	client             *client.Client
	clusterOperation   *commonsv1alpha1.ClusterOperationSpec
	image              *opgoutil.Image
	zkConfigMapName    string
	authenticationSpec []dolphinv1alpha1.AuthenticationSpec
}

// Buile implements common.RoleReconcilerBuilder.
// api server role has resources below:
// - deployment
// - service
func (a *ApierRoleResourceReconcilerBuilder) ResourceReconcilers(ctx context.Context, roleGroupInfo *reconciler.RoleGroupInfo,
	mergedCfg *dolphinv1alpha1.RoleGroupSpec) []reconciler.Reconciler {
	var reconcilers []reconciler.Reconciler

	//Configmap
	apiServerConfigMap := common.NewConfigMapReconciler(ctx, a.client, roleGroupInfo, MainContainerName, mergedCfg)
	reconcilers = append(reconcilers, apiServerConfigMap)

	//deployment
	containerBuilder := common.NewContainerBuilder(MainContainerName, a.image, a.zkConfigMapName, roleGroupInfo, mergedCfg).CommonCommandArgs().
		WithPorts(util.SortedMap{
			dolphinv1alpha1.ApiPortName:       dolphinv1alpha1.ApiPort,
			dolphinv1alpha1.ApiPythonPortName: dolphinv1alpha1.ApiPythonPort,
		}).
		WithEnvs(util.SortedMap{"JAVA_OPTS": "-Xms512m -Xmx512m -Xmn256m"}).
		WithReadinessAndLivenessProbe(dolphinv1alpha1.ApiPort).
		CommonCommandArgs().
		WithVolumeMounts(nil)
	// authentication
	volumes, err := a.authentication(ctx, a.client, a.authenticationSpec, containerBuilder)
	if err != nil {
		return nil
	}
	dep := common.CreateDeploymentReconciler(containerBuilder, ctx, a.client, a.image, a.clusterOperation, roleGroupInfo, mergedCfg, a.zkConfigMapName, volumes)
	reconcilers = append(reconcilers, dep)

	//svc
	svc := common.NewServiceReconciler(a.client, common.RoleGroupServiceName(roleGroupInfo), false, nil, map[string]int32{
		dolphinv1alpha1.ApiPortName:       dolphinv1alpha1.ApiPort,
		dolphinv1alpha1.ApiPythonPortName: dolphinv1alpha1.ApiPythonPort,
	}, roleGroupInfo.GetLabels(), roleGroupInfo.GetAnnotations())
	reconcilers = append(reconcilers, svc)

	return reconcilers
}

// authentication adds authentication configuration to the apier container.
// It resolves the AuthenticationClass based on the provider in the
// AuthenticationClass, and generates the configuration for the apier.
// Supported providers are LDAP and OIDC.
// For OIDC, only Keycloak is supported.
// The function also mounts the ldap volume if LDAP is used.
func (a *ApierRoleResourceReconcilerBuilder) authentication(
	ctx context.Context,
	client *client.Client,
	authSpec []dolphinv1alpha1.AuthenticationSpec,
	containerBuilder *common.ContainerBuilder) (volumes []corev1.Volume, err error) {
	if a.authenticationSpec == nil {
		return
	}
	var authResult security.AuthenticationResult
	authResult, err = security.Authentication(ctx, client, authSpec)
	if err != nil {
		return
	}
	// add authentication envs
	authEnvs := authResult.Config
	var sortAuthEnvs util.SortedMap = make(util.SortedMap)
	for k, v := range authEnvs {
		sortAuthEnvs[k] = v
	}
	containerBuilder.WithEnvs(sortAuthEnvs)

	// add authentication secrets
	// currently only support oidc user credentials
	for _, secret := range authResult.CredintialsSecrets {
		containerBuilder.WithSecretEnvFrom(secret)
	}

	// ldap volume and volume mount
	if authResult.LdapVolume != nil && authResult.LdapVolumeMount != nil {
		containerBuilder.WithVolumeMounts(util.SortedMap{authResult.LdapVolume.Name: authResult.LdapVolumeMount.MountPath}) //with ldap volume mount
		containerBuilder.ResetCommandArgs(a.apiServerCommandArgs(containerBuilder))                                         // with ldap user credentials into command args
		volumes = append(volumes, *authResult.LdapVolume)                                                                   //with ldap volume
	}
	return
}

// insert ldap user credentials into command args, by export the env
func (a *ApierRoleResourceReconcilerBuilder) apiServerCommandArgs(
	containerbuilder *common.ContainerBuilder) string {
	args := containerbuilder.GetCommandArgs()
	var apiServerCommandArgs []string = make([]string, 0)
	apiServerCommandArgs = append(apiServerCommandArgs, security.ExtractLdapCredintialsAndExportCommand())
	apiServerCommandArgs = append(apiServerCommandArgs, args...)
	return strings.Join(apiServerCommandArgs, "\n")
}
